package spoe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeHeaders(t *testing.T) {
	headers := make([]byte, 1024)
	pos := 0

	// add a first header
	n, err := encodeString(headers[pos:], "X-Foo")
	require.NoError(t, err)
	pos += n
	n, err = encodeString(headers[pos:], "bar")
	require.NoError(t, err)
	pos += n

	// a second one, non canonical form
	n, err = encodeString(headers[pos:], "x-foo2")
	require.NoError(t, err)
	pos += n
	n, err = encodeString(headers[pos:], "bar2")
	require.NoError(t, err)
	pos += n

	// termination sequence
	n, err = encodeString(headers[pos:], "")
	require.NoError(t, err)
	pos += n
	n, err = encodeString(headers[pos:], "")
	require.NoError(t, err)
	pos += n

	headers = headers[:pos]

	res, err := DecodeHeaders(headers)
	require.NoError(t, err)

	require.Equal(t, 2, len(res))
	require.Equal(t, "bar", res.Get("X-Foo"))
	require.Equal(t, "bar2", res.Get("X-Foo2"))
}
