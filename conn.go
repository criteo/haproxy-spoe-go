package spoe

import (
	"bufio"
	"fmt"
	"net"
	"sync"

	log "github.com/sirupsen/logrus"
)

type conn struct {
	net.Conn

	handler   Handler
	buff      *bufio.ReadWriter
	frameSize int

	engineID string
}

func (c *conn) run(a *Agent) error {
	defer c.Close()

	done := make(chan struct{})
	defer close(done)

	myframe, err := decodeFrame(c, make([]byte, maxFrameSize))
	if err != nil {
		return err
	}

	if myframe.ftype != frameTypeHaproxyHello {
		return fmt.Errorf("unexpected frame type %x when initializing connection", myframe.ftype)
	}

	myframe, capabilities, healcheck, err := c.handleHello(myframe)
	if err != nil {
		return err
	}

	acksKey := acksKey{
		FrameSize: c.frameSize,
		Engine:    c.engineID,
	}
	if !capabilities[capabilityAsync] {
		acksKey.Conn = c.Conn
	}

	err = encodeFrame(c.buff, myframe)
	if err != nil {
		return err
	}
	err = c.buff.Flush()
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
				err = encodeFrame(c.buff, myframe)
				if err != nil {
					log.Errorf("spoe: %s", err)
					continue
				}
				err = c.buff.Flush()
				if err != nil {
					log.Errorf("spoe: %s", err)
					continue
				}
				pool.Put(myframe.data[:c.frameSize])
			}
		}
	}()

	for {
		myframe, err := decodeFrame(c, pool.Get().([]byte))
		if err != nil {
			return err
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
				log.Errorf("spoe: %s", err)
			}
			return nil

		default:
			log.Errorf("spoe: frame type %x is not handled", myframe.ftype)
		}
	}
}
