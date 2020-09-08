package spoe

import (
	"fmt"

	"github.com/pkg/errors"
)

func (c *conn) disconnectFrame(e spoeError) (frame, error) {
	f := frame{
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

func (c *conn) handleDisconnect(f frame) error {
	data, _, err := decodeKVs(f.data, -1)
	if err != nil {
		return errors.Wrap(err, "disconnect")
	}

	code, ok := data["status-code"].(uint32)
	if !ok || code != 0 {
		message, _ := data["message"].(string)
		if message == "" {
			message = "unknown error "
		}

		return fmt.Errorf("disconnect error: %s", message)
	}

	return nil
}
