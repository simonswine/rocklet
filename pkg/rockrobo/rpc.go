package rockrobo

import (
	"net"
	"os"

	"github.com/simonswine/rocklet/pkg/rockrobo/rpc"
)

func (r *Rockrobo) runRPCServer() error {
	rpcInterface := rpc.New(r, r.logger)
	socketPath := rpcInterface.SocketPath
	log := r.logger.With().Str("server", "rpc").Str("socket_path", socketPath).Logger()
	s := rpc.NewServer(rpcInterface)

	err := os.Remove(socketPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	unixListener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	log.Debug().Msg("successfully bound")

	// close listener if necessary
	r.waitGroup.Add(1)
	go func() {
		defer r.waitGroup.Done()

		<-r.stopCh
		err := unixListener.Close()
		if err != nil {
			r.logger.Debug().Msgf("error stopping rpc server: %s", err)
		}
		log.Debug().Msg("stopped")
	}()

	// handle connections
	r.waitGroup.Add(1)
	go func() {
		defer r.waitGroup.Done()
		for {
			fd, err := unixListener.Accept()
			if err != nil {
				if x, ok := err.(*net.OpError); ok && x.Err.Error() == "use of closed network connection" {
					break
				}
				log.Error().Err(err).Msg("failed to accept unix socket")
			}

			// handle new connection in new go routine
			go s.ServeConn(fd)
		}
	}()

	return nil

}
