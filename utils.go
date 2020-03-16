package spoe

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

func DecodeHeaders(headers []byte) (http.Header, error) {
	res := make(http.Header)
	pos := 0
	for pos < len(headers) {
		key, n, err := decodeString(headers[pos:])
		if err != nil {
			return nil, errors.Wrap(err, "error decoding headers")
		}
		pos += n

		value, n, err := decodeString(headers[pos:])
		if err != nil {
			return nil, errors.Wrap(err, "error decoding headers")
		}
		pos += n

		if key == "" && value == "" {
			if pos != len(headers) {
				return nil, fmt.Errorf("error decoding headers: received empty values before end of buffer")
			}
			break
		}

		res.Add(key, value)
	}

	return res, nil
}
