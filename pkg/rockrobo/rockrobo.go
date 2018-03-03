package rockrobo

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/rs/zerolog"
)

type Rockrobo struct {
	GlobalBindAddr   string
	globalConnection net.PacketConn

	LocalBindAddr string
	localListener net.Listener

	stopCh chan struct{}

	logger zerolog.Logger

	outgoingQueue      chan interface{}
	incomingQueues     map[string]chan *methodMsg
	incomingQueuesLock sync.Mutex

	deviceID    *string
	deviceToken *string
}

func New() *Rockrobo {
	r := &Rockrobo{
		LocalBindAddr:  "127.0.0.1:54322",
		GlobalBindAddr: "0.0.0.0:54321",
		logger: zerolog.New(os.Stdout).With().
			Str("app", "sucklet").
			Logger().Level(zerolog.DebugLevel),

		incomingQueues: make(map[string]chan *methodMsg),
		outgoingQueue:  make(chan interface{}, 0),
	}

	return r
}

func (r *Rockrobo) Logger() *zerolog.Logger {
	return &r.logger
}

func (r *Rockrobo) Listen() (err error) {
	// listen for local connections
	r.globalConnection, err = net.ListenPacket("udp", r.GlobalBindAddr)
	if err != nil {
		return fmt.Errorf("error binding global socket: %s", err)
	}
	r.Logger().Debug().Msgf("successfully bound global server to %s", r.GlobalBindAddr)

	// listen for local connections
	r.localListener, err = net.Listen("tcp", r.LocalBindAddr)
	if err != nil {
		return fmt.Errorf("error binding local socket: %s", err)
	}
	r.Logger().Debug().Msgf("successfully bound local server to %s", r.LocalBindAddr)

	return nil
}

func (r *Rockrobo) GetDeviceID() (*string, error) {
	dataCh := make(chan *methodMsg)

	// setup, remove channel callback
	r.incomingQueuesLock.Lock()
	r.incomingQueues["_internal.response_dinfo"] = dataCh
	r.incomingQueuesLock.Unlock()
	defer func() {
		r.incomingQueuesLock.Lock()
		delete(r.incomingQueues, "_internal.response_dinfo")
		r.incomingQueuesLock.Unlock()
	}()

	// write message
	r.outgoingQueue <- &RequestDeviceID{
		Method: RequestDeviceIDMethod,
		Params: "/mnt/data/miio/",
	}

	// wait for response
	// TODO add timeout
	resp := <-dataCh

	r.Logger().Debug().Interface("resp", resp).Msg("data received")

	return nil, nil

}

func (r *Rockrobo) Run() error {
	// bind sockets
	err := r.Listen()
	if err != nil {
		return err
	}

	// run tcp connection handler
	go func() {
		for {
			//accept connections using Listener.Accept()
			conn, err := r.localListener.Accept()
			if err != nil {
				r.logger.Warn().Err(err).Msg("accepting local connection failed")
				continue
			}
			//It's common to handle accepted connection on different goroutines
			c := r.newLocalConnection(conn)
			go c.handle()
		}
	}()

	// run udp receiver

	// run status loop
	r.GetDeviceID()
	r.GetDeviceID()

	// wait for exist signal
	<-r.stopCh

	return nil
}
