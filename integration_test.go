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
	"syscall"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const haproxyConfig = `
global
	# debug
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

listen stats
    mode http
    bind *:8001
    stats uri /
    stats admin if TRUE
    stats refresh 10s

frontend spoe-test-frontend
	bind *:10080
	mode http
	filter spoe engine spoe-mirror config %s/spoe.cfg
	filter spoe engine spoe-mirror-2 config %s/spoe.cfg
	http-request deny unless { var(sess.spoe.spoe_ok) -m int eq 1 }
	default_backend servers-backend

backend servers-backend
	mode http
	server server-1 %s

backend spoe-mirror-backend
	mode tcp
	balance roundrobin
	timeout connect 5s
	timeout server 1m
	server spoe-test-server-1 %s
`
const haproxySpoeConfig = `
[spoe-mirror]
spoe-agent mirror
	# option set-on-error     err
	# option set-process-time ptime
	option var-prefix       spoe
	# option set-total-time   ttime
	messages mirror

	timeout hello      500ms
	timeout idle       35s
	timeout processing 100ms
	use-backend spoe-mirror-backend

spoe-message mirror
	args ip=src cert=ssl_c_der
	event on-frontend-http-request

[spoe-mirror-2]
spoe-agent mirror-2
	# option set-on-error     err
	# option set-process-time ptime
	option var-prefix       spoe
	# option set-total-time   ttime
	messages mirror

	timeout hello      500ms
	timeout idle       35s
	timeout processing 100ms
	use-backend spoe-mirror-backend

spoe-message mirror
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

	var requestsLock sync.Mutex
	requests := 0

	go http.Serve(originSock, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		requestsLock.Lock()
		requests++
		requestsLock.Unlock()
		rw.Write([]byte("ok"))
	}))

	spoeSock, err := net.Listen("unix", filepath.Join(cfgDir, "spoe.sock"))
	require.NoError(t, err)
	defer spoeSock.Close()

	var spoeMessagesLock sync.Mutex
	spoeMessages := 0

	spoe := New(func(msgs *MessageIterator) ([]Action, error) {
		for msgs.Next() {
			spoeMessagesLock.Lock()
			spoeMessages++
			spoeMessagesLock.Unlock()
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

	reqCount := 50000
	parallel := 10

	var wg sync.WaitGroup
	wg.Add(parallel)

	for j := 0; j < parallel; j++ {
		go func() {
			defer wg.Done()
			for i := 0; i < reqCount; i++ {
				res, err := http.Get("http://127.0.0.1:10080")
				if err != nil {
					fmt.Println(err)
				}
				require.NoError(t, err)
				if res == nil {
					continue
				}

				res.Body.Close()
				require.Equal(t, 200, res.StatusCode)
			}
		}()
	}

	wg.Wait()

	spoeMessagesLock.Lock()
	require.Equal(t, reqCount*parallel, spoeMessages)
	spoeMessagesLock.Unlock()

	requestsLock.Lock()
	require.Equal(t, reqCount*parallel, requests)
	requestsLock.Unlock()
}
