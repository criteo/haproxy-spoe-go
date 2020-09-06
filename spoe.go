package spoe

import (
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"
)

const (
	version      = "2.0"
	maxFrameSize = 16380
)

type Handler func(msgs *MessageIterator) ([]Action, error)

type Config struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

var defaultConfig = Config{
	ReadTimeout:  time.Second,
	WriteTimeout: time.Second,
	IdleTimeout:  30 * time.Second,
}

type acksKey struct {
	FrameSize int
	Engine    string
	Conn      net.Conn
}

type Agent struct {
	Handler Handler
	cfg     Config

	maxFrameSize int

	acksLock sync.Mutex
	acks     map[acksKey]chan frame
	acksWG   map[acksKey]*sync.WaitGroup
}

func New(h Handler) *Agent {
	return NewWithConfig(h, defaultConfig)
}

func NewWithConfig(h Handler, cfg Config) *Agent {
	return &Agent{
		Handler: h,
		cfg:     cfg,
		acks:    make(map[acksKey]chan frame),
		acksWG:  make(map[acksKey]*sync.WaitGroup),
	}
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
			return err
		}

		err = c.(*net.TCPConn).SetWriteBuffer(maxFrameSize * 4)
		if err != nil {
			return err
		}
		err = c.(*net.TCPConn).SetReadBuffer(maxFrameSize * 4)
		if err != nil {
			return err
		}

		log.Debugf("spoe: connection from %s", c.RemoteAddr())

		go func() {
			c := &conn{
				Conn:        c,
				handler:     a.Handler,
				cfg:         a.cfg,
				notifyTasks: make(chan frame),
			}
			err := c.run(a)
			if err != nil {
				log.Errorf("spoe: error handling connection: %s", err)
			}
		}()
	}
}
