package spoe

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"

	gerrs "errors"

	pool "github.com/libp2p/go-buffer-pool"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type frameType byte

const (
	frameTypeUnset frameType = 0

	// Frames sent by HAProxy
	frameTypeHaproxyHello  frameType = 1
	frameTypeHaproxyDiscon frameType = 2
	frameTypeHaproxyNotify frameType = 3

	// Frames sent by the agents
	frameTypeAgentHello  frameType = 101
	frameTypeAgentDiscon frameType = 102
	frameTypeAgentACK    frameType = 103
)

type frameFlag uint32

const (
	frameFlagFin  = 1
	frameFlagAbrt = 2
)

type Frame struct {
	ftype        frameType
	flags        frameFlag
	streamID     int
	frameID      int
	data         []byte
	originalData []byte
}

type codec struct {
	conn net.Conn
	buff *bufio.ReadWriter
	cfg  Config
}

func newCodec(conn net.Conn, cfg Config) *codec {
	return &codec{
		conn: conn,
		buff: bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		cfg:  cfg,
	}
}

func (c *codec) decodeFrame(frame *Frame) (bool, error) {
	buffer := pool.Get(maxFrameSize)
	frame.originalData = buffer

	err := c.conn.SetReadDeadline(time.Now().Add(c.cfg.IdleTimeout))
	if err != nil {
		return false, errors.Wrap(err, "frame read")
	}

	// read the frame length
	_, err = io.ReadFull(c.buff, buffer[:4])
	// EOF on first read is not an error
	if err == io.EOF {
		return false, nil
	}
	// special case for idle timeout
	if gerrs.Is(err, os.ErrDeadlineExceeded) {
		log.Debug("spoe: connection idle timeout")
		return false, nil
	}

	if err != nil {
		if errors.Is(err, syscall.ECONNRESET) {
			return false, nil
		}
		return false, errors.Wrap(err, "frame read")
	}

	// we have a frame, switch to read timeout
	err = c.conn.SetDeadline(time.Now().Add(c.cfg.ReadTimeout))
	if err != nil {
		return false, errors.Wrap(err, "frame read")
	}

	frameLength, _, err := decodeUint32(buffer[:4])
	if err != nil {
		return false, errors.Wrap(err, "frame read")
	}

	if frameLength > maxFrameSize {
		return false, errors.New("frame length")
	}

	frame.data = buffer[:frameLength]

	// read the frame data
	_, err = io.ReadFull(c.buff, frame.data)
	if err != nil {
		return false, errors.Wrap(err, "frame read")
	}

	off := 0
	if len(frame.data) == 0 {
		return false, fmt.Errorf("frame read: empty frame")
	}

	frame.ftype = frameType(frame.data[0])
	off++

	flags, n, err := decodeUint32(frame.data[off:])
	if err != nil {
		return false, errors.Wrap(err, "frame read")
	}

	off += n
	frame.flags = frameFlag(flags)

	streamID, n, err := decodeVarint(frame.data[off:])
	if err != nil {
		return false, errors.Wrap(err, "frame read")
	}
	off += n

	frameID, n, err := decodeVarint(frame.data[off:])
	if err != nil {
		return false, errors.Wrap(err, "frame read")
	}
	off += n

	frame.streamID = streamID
	frame.frameID = frameID
	frame.data = frame.data[off:]
	return true, nil
}

func (c *codec) encodeFrame(f Frame) error {
	if f.originalData != nil {
		defer pool.Put(f.originalData)
	}

	err := c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
	if err != nil {
		return errors.Wrap(err, "disconnect")
	}

	header := pool.Get(17)
	defer pool.Put(header)

	off := 4

	header[off] = byte(f.ftype)
	off++

	binary.BigEndian.PutUint32(header[off:], uint32(f.flags))
	off += 4

	n, err := encodeVarint(header[off:], f.streamID)
	if err != nil {
		return errors.Wrap(err, "write frame")
	}
	off += n

	n, err = encodeVarint(header[off:], f.frameID)
	if err != nil {
		return errors.Wrap(err, "write frame")
	}
	off += n

	binary.BigEndian.PutUint32(header, uint32(off-4+len(f.data)))

	_, err = c.buff.Write(header[:off])
	if err != nil {
		return errors.Wrap(err, "write frame")
	}

	_, err = c.buff.Write(f.data)
	if err != nil {
		return errors.Wrap(err, "write frame")
	}

	err = c.buff.Flush()
	if err != nil {
		return errors.Wrap(err, "write frame")
	}

	return nil
}
