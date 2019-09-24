package spoe

import (
	"bufio"
	"net"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"
)

const (
	version      = "2.0"
	maxFrameSize = 16380
)

type Handler func(args []Message) ([]Action, error)

type acksKey struct {
	FrameSize int
	Engine    string
	Conn      net.Conn
}

type Agent struct {
	Handler Handler

	maxFrameSize int

	acksLock sync.Mutex
	acks     map[acksKey]chan frame
	acksWG   map[acksKey]*sync.WaitGroup
}

func New(h Handler) *Agent {
	a := &Agent{
		Handler: h,
		acks:    make(map[acksKey]chan frame),
		acksWG:  make(map[acksKey]*sync.WaitGroup),
	}
	return a
}

func (a *Agent) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "spoe")
	}

	return a.Serve(lis)
}

func (a *Agent) Serve(lis net.Listener) error {
	log.Infof("spoe: listening on %s", lis.Addr().String())

	for {
		c, err := lis.Accept()
		if err != nil {
			log.Errorf("spoe: %s", err)
			continue
		}

		log.Debugf("spoe: connection from %s", c.RemoteAddr())

		go func() {
			c := &conn{
				Conn:    c,
				handler: a.Handler,
				buff:    bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c)),
			}
			err := c.run(a)
			if err != nil {
				log.Errorf("spoe: error handling connection: %s", err)
			}
		}()
	}
}
