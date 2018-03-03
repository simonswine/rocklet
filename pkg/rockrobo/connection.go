package rockrobo

import (
	"encoding/json"
	"io"
	"net"
	"sync"

	"github.com/rs/zerolog"
)

const MethodHello = "_internal.hello"

type methodMsg struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type localConnection struct {
	rockrobo *Rockrobo
	conn     net.Conn
	logger   zerolog.Logger
	closeCh  chan struct{} // channel that is closed a soon the other side closes the connection
}

func (r *Rockrobo) newLocalConnection(conn net.Conn) *localConnection {
	return &localConnection{
		conn:     conn,
		rockrobo: r,
		logger:   r.Logger().With().Str("remote_addr", conn.RemoteAddr().String()).Logger(),
		closeCh:  make(chan struct{}),
	}
}

func (c *localConnection) handle() {
	c.logger.Debug().Msg("accepted local connection")
	defer func() {
		c.logger.Debug().Msg("closed local connection")
		c.conn.Close()
	}()

	// setup waitgroup
	wg := sync.WaitGroup{}

	// channel to signal readiness
	readyCh := make(chan struct{})

	// read incoming data
	wg.Add(1)
	go func() {
		d := json.NewDecoder(c.conn)
		d.DisallowUnknownFields()
		defer func() {
			close(c.closeCh)
			wg.Done()
		}()

		readySent := false

		for {
			var method methodMsg
			err := d.Decode(&method)

			if err == io.EOF {
				c.logger.Debug().Msg("received EOF")
				return
			} else if err != nil {
				c.logger.Warn().Err(err).Msg("error reading local connection")
				return
			}
			c.logger.Debug().Interface("data", method).Msg("received data on local connection")

			// if hello received for the frist time, signal readiness
			if method.Method == MethodHello {
				if !readySent {
					close(readyCh)
					readySent = true
				}
				continue
			}

			// retrieve correct channel
			c.rockrobo.incomingQueuesLock.Lock()
			dataCh, ok := c.rockrobo.incomingQueues[method.Method]
			c.rockrobo.incomingQueuesLock.Unlock()

			if !ok {
				c.logger.Warn().Interface("method", method.Method).Msg("no channel to handle this method")
				continue
			}

			// deliver message in channel
			go func() {
				dataCh <- &method
			}()
		}
	}()

	// write outgoing data
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-readyCh

		e := json.NewEncoder(c.conn)

		c.logger.Debug().Msg("local connection ready")
		for {
			select {
			case obj := <-c.rockrobo.outgoingQueue:
				err := e.Encode(obj)
				if err != nil {
					c.logger.Warn().Err(err).Interface("data", obj).Msg("error sending data")
				}
				c.logger.Debug().Interface("data", obj).Msg("send data")

			case <-c.closeCh:
				c.logger.Debug().Msg("break outgoing as connection closed")
				return
			}
		}

	}()

	wg.Wait()

}
