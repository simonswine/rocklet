package rockrobo

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

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
	incomingQueues     map[string]chan *Method
	incomingQueuesLock sync.Mutex

	deviceID *MethodParamsResponseDeviceID

	rand   *rand.Rand
	nToken []byte
	dToken []byte
	dir    string
}

func New() *Rockrobo {
	r := &Rockrobo{
		LocalBindAddr:  "127.0.0.1:54322",
		GlobalBindAddr: "0.0.0.0:54321",
		logger: zerolog.New(os.Stdout).With().
			Str("app", "sucklet").
			Logger().Level(zerolog.DebugLevel),

		incomingQueues: make(map[string]chan *Method),
		outgoingQueue:  make(chan interface{}, 0),
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
		dir:            "/mnt/data/miio/",
	}

	// generate new token
	r.nToken = make([]byte, 12)
	_, err := r.rand.Read(r.nToken)
	if err != nil {
		panic(err)
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

func (r *Rockrobo) GetDeviceID() (*MethodParamsResponseDeviceID, error) {
	// retrieve device ID
	response, err := r.retrieve(
		&Method{
			Method: MethodRequestDeviceID,
			Params: json.RawMessage(fmt.Sprintf(`"%s"`, r.dir)),
		},
		MethodResponseDeviceID,
	)
	if err != nil {
		return nil, err
	}

	var params MethodParamsResponseDeviceID
	err = json.Unmarshal(response.Params, &params)
	if err != nil {
		return nil, err
	}

	return &params, nil

}

func (r *Rockrobo) GetToken() (token []byte, err error) {
	// marshall params
	reqParams, err := json.Marshal(&MethodParamsRequestToken{
		Dir:    r.dir,
		NToken: r.nToken,
	})

	// retrieve token
	response, err := r.retrieve(
		&Method{
			Method: MethodRequestToken,
			Params: reqParams,
		},
		MethodResponseToken,
	)
	if err != nil {
		return []byte{}, err
	}

	err = json.Unmarshal(response.Params, &token)
	if err != nil {
		return []byte{}, err
	}

	return token, nil
}

func (r *Rockrobo) retrieve(requestObj interface{}, methodResponse string) (*Method, error) {
	dataCh := make(chan *Method)

	// setup, remove channel callback
	r.incomingQueuesLock.Lock()
	r.incomingQueues[methodResponse] = dataCh
	r.incomingQueuesLock.Unlock()
	defer func() {
		r.incomingQueuesLock.Lock()
		delete(r.incomingQueues, methodResponse)
		r.incomingQueuesLock.Unlock()
	}()

	// write message
	r.outgoingQueue <- requestObj

	// wait for response
	// TODO add timeout
	resp := <-dataCh

	r.Logger().Debug().Interface("resp", resp).Msg("data received")

	return resp, nil
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
	r.deviceID, err = r.GetDeviceID()
	if err != nil {
		return err
	}

	r.dToken, err = r.GetToken()
	if err != nil {
		return err
	}

	// wait for exist signal
	<-r.stopCh

	return nil
}
