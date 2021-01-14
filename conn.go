package spoe

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

const workerIdleTimeout = 2 * time.Second

type conn struct {
	net.Conn
	cfg Config

	handler   Handler
	frameSize int

	engineID string

	notifyTasks chan frame
}

func (c *conn) run(a *Agent) error {
	defer c.Close()

	done := make(chan struct{})
	defer close(done)

	cod := newCodec(c.Conn, c.cfg)

	myframe := frame{}
	ok, err := cod.decodeFrame(&myframe)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	if myframe.ftype != frameTypeHaproxyHello {
		return fmt.Errorf("unexpected frame type %x when initializing connection", myframe.ftype)
	}

	myframe, capabilities, healcheck, err := c.handleHello(myframe)
	if err != nil {
		return err
	}

	disconnError := spoeErrorNone
	defer func() {
		df, err := c.disconnectFrame(disconnError)
		if err != nil {
			log.Errorf("spoe: %s", err)
			return
		}

		err = cod.encodeFrame(df)
		if err != nil {
			log.Errorf("spoe: %s", err)
			return
		}
	}()

	acksKey := acksKey{
		FrameSize: c.frameSize,
		Engine:    c.engineID,
	}
	if !capabilities[capabilityAsync] {
		acksKey.Conn = c.Conn
	}

	err = cod.encodeFrame(myframe)
	if err != nil {
		return err
	}
	if healcheck {
		return nil
	}

	defer func() {
		a.acksLock.Lock()
		defer a.acksLock.Unlock()

		new := atomic.AddInt32(&a.acks[acksKey].count, -1)
		if new == 0 {
			delete(a.acks, acksKey)
		}
	}()

	var acks chan frame
	a.acksLock.Lock()
	eng, ok := a.acks[acksKey]
	if ok {
		acks = eng.acks
		atomic.AddInt32(&eng.count, 1)
	} else {
		a.acks[acksKey] = &engine{
			acks:  make(chan frame),
			count: 1,
		}
	}
	a.acksLock.Unlock()

	// run reply loop
	go func() {
		for {
			select {
			case <-done:
				return
			case frame := <-acks:
				err = cod.encodeFrame(frame)
				if err != nil {
					log.Errorf("spoe: %s", err)
					continue
				}
			}
		}
	}()

	for {
		ok, err := cod.decodeFrame(&myframe)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		switch myframe.ftype {
		case frameTypeHaproxyNotify:
			select {
			case c.notifyTasks <- myframe:
			default:
				go c.runWorker(myframe, acks)
			}

		case frameTypeHaproxyDiscon:
			err := c.handleDisconnect(myframe)
			if err != nil {
				return err
			}
			return nil

		default:
			return fmt.Errorf("spoe: frame type %x is not handled", myframe.ftype)
		}
	}
}

func (c *conn) runWorker(f frame, acks chan frame) {
	err := c.handleNotify(f, acks)
	if err != nil {
		log.Errorf("spoe: %s", err)
	}
	timeout := time.NewTimer(workerIdleTimeout)

	for {
		select {
		case f := <-c.notifyTasks:
			err := c.handleNotify(f, acks)
			if err != nil {
				log.Errorf("spoe: %s", err)
			}
			timeout.Reset(workerIdleTimeout)
		case <-timeout.C:
			return
		}

	}
}
