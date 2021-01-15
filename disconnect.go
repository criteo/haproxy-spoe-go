package spoe

import (
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func (c *conn) disconnectFrame(e spoeError) (Frame, error) {
	f := Frame{
		frameID:  0,
		streamID: 0,
		ftype:    frameTypeAgentDiscon,
		flags:    frameFlagFin,
		data:     make([]byte, c.frameSize),
	}

	off := 0

	n, err := encodeKV(f.data[off:], "status-code", int(e))
	if err != nil {
		return f, errors.Wrap(err, "disconnect")
	}
	off += n

	n, err = encodeKV(f.data[off:], "message", spoeErrorMessages[e])
	if err != nil {
		return f, errors.Wrap(err, "disconnect")
	}
	off += n

	f.data = f.data[:off]

	return f, nil
}

func (c *conn) handleDisconnect(f Frame) error {
	data, _, err := decodeKVs(f.data, -1)
	log.Debugf("spoe: Disconnect from %s: %+v", c.Conn.RemoteAddr(), data)
	if err != nil {
		return errors.Wrap(err, "disconnect")
	}

	code, ok := data["status-code"].(uint)
	if !ok {
		message, _ := data["message"].(string)
		if message == "" {
			message = "unknown error "
		}
		return fmt.Errorf("disconnect error without status-code and message: %s", message)
	}

	if spoeError(code) == spoeErrorTimeout || spoeError(code) == spoeErrorNone {
		return nil
	}

	message, ok_message := spoeErrorMessages[spoeError(code)]
	if ok_message {
		return fmt.Errorf("disconnect error: %s", message)
	}

	message, _ = data["message"].(string)
	if message == "" {
			message = "unknown error "
	}
	return fmt.Errorf("disconnect error (without status-code) : %s", message)
}
