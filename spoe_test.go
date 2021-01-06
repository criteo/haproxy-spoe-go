package spoe

import (
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestSPOE(t *testing.T) {
	spoa := New(func(msgs *MessageIterator) ([]Action, error) {
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

	helloRes := frame{}
	ok, err := cod.decodeFrame(&helloRes)
	require.True(t, ok)
	require.NoError(t, err)

	require.Equal(t, helloReq.streamID, helloRes.streamID)

	// notify
	notifyReq := notifyFrame(t)
	require.NoError(t, cod.encodeFrame(notifyReq))

	notifyRes := frame{}
	ok, err = cod.decodeFrame(&notifyRes)
	require.True(t, ok)
	require.NoError(t, err)

	require.Equal(t, notifyReq.streamID, notifyRes.streamID)
}

func TestSPOEUnix(t *testing.T) {
	spoa := New(func(msgs *MessageIterator) ([]Action, error) {
		return nil, nil
	})

	name, err := ioutil.TempDir("/tmp", "http-mirror-test")
	require.NoError(t, err)
	defer os.RemoveAll(name)

	sock := filepath.Join(name, "spoe.sock")

	lis, err := net.Listen("unix", sock)
	require.NoError(t, err)
	defer lis.Close()

	agentError := make(chan error)
	go func() {
		agentError <- spoa.Serve(lis)
	}()

	client, err := net.Dial("unix", sock)
	require.NoError(t, err)

	cod := newCodec(client, defaultConfig)

	// hello
	helloReq := helloFrame(t)
	require.NoError(t, cod.encodeFrame(helloReq))

	helloRes := frame{}
	ok, err := cod.decodeFrame(&helloRes)
	require.True(t, ok)
	require.NoError(t, err)

	require.Equal(t, helloReq.streamID, helloRes.streamID)

	// notify
	notifyReq := notifyFrame(t)
	require.NoError(t, cod.encodeFrame(notifyReq))

	notifyRes := frame{}
	ok, err = cod.decodeFrame(&notifyRes)
	require.True(t, ok)
	require.NoError(t, err)

	require.Equal(t, notifyReq.streamID, notifyRes.streamID)
}

func BenchmarkSPOE(b *testing.B) {
	log.SetLevel(log.FatalLevel)
	spoa := New(func(msgs *MessageIterator) ([]Action, error) {
		for msgs.Next() {
		}
		return nil, nil
	})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(b, err)

	agentError := make(chan error)
	go func() {
		agentError <- spoa.Serve(lis)
	}()

	client, err := net.Dial("tcp", lis.Addr().String())
	require.NoError(b, err)

	cod := newCodec(client, defaultConfig)

	// hello
	helloReq := helloFrame(b)
	require.NoError(b, cod.encodeFrame(helloReq))

	notifyReq := notifyFrame(b)
	res := frame{}

	ok, err := cod.decodeFrame(&res)
	require.True(b, ok)
	require.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := cod.encodeFrame(notifyReq)
		if err != nil {
			b.Fatal(err)
		}
		_, err = cod.decodeFrame(&res)
		if err != nil {
			b.Error(err)
		}
	}
}
