package spoe

import (
	"github.com/stretchr/testify/require"
)

func notifyFrame(t require.TestingT) frame {
	b := make([]byte, maxFrameSize)
	f := frame{
		ftype:    frameTypeHaproxyNotify,
		streamID: 1,
		frameID:  2,
		data:     b,
	}

	m := 0

	n, err := encodeString(f.data[m:], "message-1")
	m += n
	f.data[m] = 2
	m++
	n, err = encodeKV(f.data[m:], "key-1", "value1")
	require.NoError(t, err)
	m += n
	n, err = encodeKV(f.data[m:], "key-2", 360)
	require.NoError(t, err)
	m += n

	n, err = encodeString(f.data[m:], "message-2")
	m += n
	f.data[m] = 2
	m++
	n, err = encodeKV(f.data[m:], "key2-1", "value21")
	require.NoError(t, err)
	m += n
	n, err = encodeKV(f.data[m:], "key2-2", 362)
	require.NoError(t, err)
	m += n

	f.data = f.data[:m]

	return f
}

func helloFrame(t require.TestingT) frame {
	f := frame{
		ftype:    frameTypeHaproxyHello,
		flags:    frameFlagFin,
		streamID: 1,
		frameID:  1,
		data:     make([]byte, maxFrameSize),
	}

	m := 0

	n, err := encodeKV(f.data[m:], "capabilities", "pipelining")
	require.NoError(t, err)
	m += n

	n, err = encodeKV(f.data[m:], "engine-id", "0082BA75-E79D-4766-86D3-1AE84AB4A366")
	require.NoError(t, err)
	m += n

	n, err = encodeKV(f.data[m:], "max-frame-size", uint(16380))
	require.NoError(t, err)
	m += n

	n, err = encodeKV(f.data[m:], "supported-versions", "2.0")
	require.NoError(t, err)
	m += n

	f.data = f.data[:m]

	return f
}
