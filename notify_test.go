package spoe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotify(t *testing.T) {
	data := make([]byte, maxFrameSize)
	f := Frame{
		data:         data,
		originalData: data,
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
		handler: func(msgs *MessageIterator) ([]Action, error) {
			ok := msgs.Next()
			require.True(t, ok)
			require.Equal(t, "message-1", msgs.Message.Name)
			require.Equal(t, 2, msgs.Message.Args.Count())
			args := msgs.Message.Args.Map()
			require.Equal(t, map[string]interface{}{
				"key-1": "value1",
				"key-2": 360,
			}, args)

			ok = msgs.Next()
			require.True(t, ok)
			require.Equal(t, "message-2", msgs.Message.Name)
			require.Equal(t, 2, msgs.Message.Args.Count())
			args = msgs.Message.Args.Map()
			require.Equal(t, map[string]interface{}{
				"key2-1": "value21",
				"key2-2": 362,
			}, args)

			require.False(t, msgs.Next())
			require.NoError(t, msgs.Error())

			handlerCalled = true

			return actions, nil
		},
	}

	out := make(chan Frame, 1)
	err = conn.handleNotify(f, out)
	require.Nil(t, err)
	require.True(t, handlerCalled)
}
