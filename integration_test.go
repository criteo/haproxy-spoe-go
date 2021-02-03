package spoe

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/phayes/freeport"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const haproxyConfig = `
global
    nbthread 64
    maxconn 5000
    hard-stop-after 10s
    stats socket /tmp/haproxy.sock mode 666 level admin

defaults
    log     global
    mode    http
    option  dontlognull
    retries 3
    maxconn 4000
    timeout connect 5000
    timeout client  50000
    timeout server  50000

frontend spoe-test-frontend
	bind *:%d
	mode http
	filter spoe engine spoe-test config %s/spoe.cfg
	filter spoe engine spoe-test-2 config %s/spoe.cfg
	http-request deny unless { var(sess.spoe.spoe_ok) -m int eq 1 }
	default_backend servers-backend

backend servers-backend
	mode http
	server server-1 %s

backend spoe-test-backend
	mode tcp
	balance roundrobin
	timeout connect 5s
	timeout server 1m
	server spoe-test-server-1 %s
`
const haproxySpoeConfig = `
[spoe-test]
spoe-agent test
	option var-prefix       spoe
	messages test

	timeout hello      500ms
	timeout idle       35s
	timeout processing 100ms
	use-backend spoe-test-backend

spoe-message test
	args ip=src cert=ssl_c_der
	event on-frontend-http-request

[spoe-test-2]
spoe-agent test-2
	option var-prefix       spoe
	messages test

	timeout hello      500ms
	timeout idle       35s
	timeout processing 100ms
	use-backend spoe-test-backend

spoe-message test
	args ip=src cert=ssl_c_der
	event on-frontend-http-request
`

func TestIntegration(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cfgDir, err := ioutil.TempDir("/tmp", "spoe-test")
	require.NoError(t, err)
	defer os.RemoveAll(cfgDir)

	originSock, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer originSock.Close()

	frontBindPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	requests := uint32(0)

	go http.Serve(originSock, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		atomic.AddUint32(&requests, 1)
		rw.Write([]byte("ok"))
	}))

	spoeSock, err := net.Listen("unix", filepath.Join(cfgDir, "spoe.sock"))
	require.NoError(t, err)
	defer spoeSock.Close()

	spoeMessages := uint32(0)

	spoe := New(func(msgs *MessageIterator) ([]Action, error) {
		for msgs.Next() {
			atomic.AddUint32(&spoeMessages, 1)
		}

		return []Action{
			ActionSetVar{
				Scope: VarScopeSession,
				Name:  "spoe_ok",
				Value: 1,
			},
		}, nil
	})

	go func() {
		spoe.Serve(spoeSock)
	}()

	err = ioutil.WriteFile(
		filepath.Join(cfgDir, "haproxy.cfg"),
		[]byte(fmt.Sprintf(
			haproxyConfig,
			frontBindPort,
			cfgDir,
			cfgDir,
			originSock.Addr().String(),
			"unix@"+filepath.Join(cfgDir, "spoe.sock"),
		)),
		0o644,
	)

	err = ioutil.WriteFile(
		filepath.Join(cfgDir, "spoe.cfg"),
		[]byte(haproxySpoeConfig),
		0o644,
	)

	haproxyPath := os.Getenv("HAPROXY")
	if haproxyPath == "" {
		haproxyPath = "haproxy"
	}

	cmd := exec.Command(haproxyPath, "-f", filepath.Join(cfgDir, "haproxy.cfg"))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err = cmd.Start()
	require.NoError(t, err)

	defer cmd.Process.Signal(syscall.SIGTERM)

	time.Sleep(time.Second)

	reqCount := 200
	parallel := 4

	var wg sync.WaitGroup
	wg.Add(parallel)

	for j := 0; j < parallel; j++ {
		go func() {
			defer wg.Done()
			for i := 0; i < reqCount; i++ {
				res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", frontBindPort))
				assert.NoError(t, err)
				if res == nil {
					continue
				}

				res.Body.Close()
				assert.Equal(t, 200, res.StatusCode)
			}
		}()
	}

	wg.Wait()

	require.Equal(t, reqCount*parallel*2, int(spoeMessages))
	require.Equal(t, reqCount*parallel, int(requests))
}
