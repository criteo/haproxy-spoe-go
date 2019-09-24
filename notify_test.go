package spoe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotify(t *testing.T) {
	f := frame{
		data: make([]byte, maxFrameSize),
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

	handlerCalled := false

	actions := []Action{
		ActionSetVar{
			Name:  "set-var-1",
			Scope: VarScopeSession,
			Value: make([]byte, 1000),
		},
		ActionSetVar{
			Name:  "set-var-2",
			Scope: VarScopeSession,
			Value: make([]byte, 1500),
		},
	}

	conn := &conn{
		frameSize: maxFrameSize,
		handler: func(args []Message) ([]Action, error) {
			require.Len(t, args, 2)

			msg := args[0]
			require.Equal(t, "message-1", msg.Name)
			require.Len(t, msg.Args, 2)
			require.Equal(t, "value1", msg.Args["key-1"])
			require.Equal(t, 360, msg.Args["key-2"])

			msg = args[1]
			require.Equal(t, "message-2", msg.Name)
			require.Len(t, msg.Args, 2)
			require.Equal(t, "value21", msg.Args["key2-1"])
			require.Equal(t, 362, msg.Args["key2-2"])

			handlerCalled = true

			return actions, nil
		},
	}

	_, err = conn.handleNotify(f)
	require.Nil(t, err)
	require.True(t, handlerCalled)
}
