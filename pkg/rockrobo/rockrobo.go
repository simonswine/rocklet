package rockrobo

import (
	"fmt"
	"net"
	"os"

	"github.com/rs/zerolog"
)

type Rockrobo struct {
	GlobalBindAddr   string
	globalConnection net.PacketConn

	LocalBindAddr string
	localListener net.Listener

	stopCh chan struct{}

	logger zerolog.Logger
}

func New() *Rockrobo {
	r := &Rockrobo{
		LocalBindAddr:  "127.0.0.1:54322",
		GlobalBindAddr: "0.0.0.0:54321",
		logger: zerolog.New(os.Stdout).With().
			Str("app", "sucklet").
			Logger().Level(zerolog.DebugLevel),
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

func (r *Rockrobo) handleLocalConnection(c net.Conn) {
	r.logger.Debug().Msgf("accepted local connection %s", c.RemoteAddr())
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
			c, err := r.localListener.Accept()
			if err != nil {
				r.logger.Warn().Err(err).Msg("accepting local connection failed")
				continue
			}
			//It's common to handle accepted connection on different goroutines
			go r.handleLocalConnection(c)
		}
	}()

	// run udp receiver

	// wait for exist signal
	<-r.stopCh

	return nil
}
