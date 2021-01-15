package spoe

import (
	"fmt"
	"net"
	"sync"
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

	notifyTasks chan Frame
}

func (c *conn) run(a *Agent) error {
	defer c.Close()

	done := make(chan struct{})
	defer close(done)

	cod := newCodec(c.Conn, c.cfg)

	myframe := Frame{}
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
			log.Warnf("spoe: %s", err)
			return
		}

		err = cod.encodeFrame(df)
		if err != nil {
			log.Errorf("spoe: %s", err)
			return
		}
	}()

	framesKey := FrameKey{
		FrameSize: c.frameSize,
		Engine:    c.engineID,
	}
	if !capabilities[capabilityAsync] {
		framesKey.Conn = c.Conn
	}

	err = cod.encodeFrame(myframe)
	if err != nil {
		return err
	}
	if healcheck {
		return nil
	}

	a.framesLock.Lock()
	if _, ok := a.frames[framesKey]; !ok {
		a.frames[framesKey] = make(chan Frame)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		a.framesWG[framesKey] = wg

		go func() {
			// wait until there is no more connection for this engine-id
			// before deleting it
			wg.Wait()

			a.framesLock.Lock()
			delete(a.framesWG, framesKey)
			delete(a.frames, framesKey)
			a.framesLock.Unlock()
		}()
	} else {
		a.framesWG[framesKey].Add(1)
	}
	// signal that this connection is done using the engine
	defer a.framesWG[framesKey].Done()

	frames := a.frames[framesKey]
	a.framesLock.Unlock()

	// run reply loop
	go func() {
		for {
			select {
			case <-done:
				return
			case frame := <-frames:
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
				go c.runWorker(myframe, frames)
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

func (c *conn) runWorker(f Frame, frames chan Frame) {
	err := c.handleNotify(f, frames)
	if err != nil {
		log.Warnf("spoe: %s", err)
	}
	timeout := time.NewTimer(workerIdleTimeout)

	for {
		select {
		case f := <-c.notifyTasks:
			err := c.handleNotify(f, frames)
			if err != nil {
				log.Warnf("spoe: %s", err)
			}
			timeout.Reset(workerIdleTimeout)
		case <-timeout.C:
			return
		}

	}
}
