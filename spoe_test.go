package spoe

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSPOE(t *testing.T) {
	spoa := New(func(args []Message) ([]Action, error) {
		return nil, nil
	})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	agentError := make(chan error)
	go func() {
		agentError <- spoa.Serve(lis)
	}()

	client, err := net.Dial("tcp", lis.Addr().String())
	require.NoError(t, err)

	cod := newCodec(client, defaultConfig)

	// hello
	helloReq := helloFrame(t)
	require.NoError(t, cod.encodeFrame(helloReq))

	helloRes, ok, err := cod.decodeFrame(make([]byte, maxFrameSize))
	require.True(t, ok)
	require.NoError(t, err)

	require.Equal(t, helloReq.streamID, helloRes.streamID)

	// notify
	notifyReq := notifyFrame(t)
	require.NoError(t, cod.encodeFrame(notifyReq))

	notifyRes, ok, err := cod.decodeFrame(make([]byte, maxFrameSize))
	require.True(t, ok)
	require.NoError(t, err)

	require.Equal(t, notifyReq.streamID, notifyRes.streamID)
}
