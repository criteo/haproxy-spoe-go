package spoe

import (
	"fmt"
	"net"
	"sync"

	log "github.com/sirupsen/logrus"
)

type conn struct {
	net.Conn
	cfg Config

	handler   Handler
	frameSize int

	engineID string
}

func (c *conn) run(a *Agent) error {
	defer c.Close()

	done := make(chan struct{})
	defer close(done)

	cod := newCodec(c.Conn, c.cfg)

	myframe, ok, err := cod.decodeFrame(make([]byte, maxFrameSize))
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

	pool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, c.frameSize)
		},
	}

	a.acksLock.Lock()
	if _, ok := a.acks[acksKey]; !ok {
		a.acks[acksKey] = make(chan frame)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		a.acksWG[acksKey] = wg

		go func() {
			// wait until there is no more connection for this engine-id
			// before deleting it
			wg.Wait()

			a.acksLock.Lock()
			delete(a.acksWG, acksKey)
			delete(a.acks, acksKey)
			a.acksLock.Unlock()
		}()
	} else {
		a.acksWG[acksKey].Add(1)
	}
	// signal that this connection is done using the engine
	defer a.acksWG[acksKey].Done()

	acks := a.acks[acksKey]
	a.acksLock.Unlock()

	// run reply loop
	go func() {
		for {
			select {
			case <-done:
				return
			case myframe := <-acks:
				err = cod.encodeFrame(myframe)
				if err != nil {
					log.Errorf("spoe: %s", err)
					continue
				}
				pool.Put(myframe.originalData)
			}
		}
	}()

	for {
		myframe, ok, err := cod.decodeFrame(pool.Get().([]byte))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		switch myframe.ftype {
		case frameTypeHaproxyNotify:
			go func() {
				myframe, err = c.handleNotify(myframe)
				if err != nil {
					log.Errorf("spoe: %s", err)
					return
				}

				acks <- myframe
			}()

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
