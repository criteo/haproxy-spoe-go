package spoe

import (
	"fmt"
	"net"
	"testing"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
)

func TestSPOE(t *testing.T) {
	spoa := New(func(args []Message) ([]Action, error) {
		return nil, nil
	})

	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	lis, err := net.Listen("tcp", addr)
	require.NoError(t, err)

	agentError := make(chan error)
	go func() {
		agentError <- spoa.Serve(lis)
	}()

	client, err := net.Dial("tcp", addr)
	require.NoError(t, err)

	// hello
	helloReq := helloFrame(t)
	require.NoError(t, encodeFrame(client, helloReq))

	helloRes, err := decodeFrame(client, make([]byte, maxFrameSize))
	require.NoError(t, err)

	require.Equal(t, helloReq.streamID, helloRes.streamID)

	// notify
	notifyReq := notifyFrame(t)
	require.NoError(t, encodeFrame(client, notifyReq))

	notifyRes, err := decodeFrame(client, make([]byte, maxFrameSize))
	require.NoError(t, err)

	require.Equal(t, notifyReq.streamID, notifyRes.streamID)
}
