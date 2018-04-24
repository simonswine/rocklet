package rockrobo

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/simonswine/rocklet/pkg/api"
	"github.com/simonswine/rocklet/pkg/metrics"
	"github.com/simonswine/rocklet/pkg/navmap"
)

const CloudDNSName = "ot.io.mi.com"

type Rockrobo struct {
	flags *api.Flags

	CloudBindAddr     string
	cloudConnection   *net.UDPConn
	cloudEndpoints    map[string]struct{} // this is a maintained map of accessbile cloud endpoints
	cloudSessions     map[int]chan []byte
	cloudSessionsLock sync.Mutex
	cloudKeepAliveCh  chan net.Addr

	LocalBindAddr string
	localListener net.Listener

	HTTPBindAddr string
	httpListener net.Listener

	// stop the run of rockrobo
	stopCh    chan struct{}
	waitGroup sync.WaitGroup

	// localDeviceReady
	localDeviceReadyCh chan struct{}

	logger zerolog.Logger

	// outgoing channel to app proxy
	outgoingQueueAppProxy chan *Method
	// outgoing channel to internal
	outgoingQueueInternal chan *Method

	incomingQueues     map[string]chan *Method
	incomingQueuesLock sync.Mutex

	deviceID     *MethodParamsResponseDeviceID
	internalInfo *MethodParamsResponseInternalInfo

	appRCSequenceNumber int

	rand   *rand.Rand
	nToken []byte
	dToken []byte
	dir    string

	// metrics module
	metrics *metrics.Metrics

	// navmap module
	navMap *navmap.NavMap
}

func New(flags *api.Flags) *Rockrobo {
	r := &Rockrobo{
		LocalBindAddr:      "127.0.0.1:54322",
		localDeviceReadyCh: make(chan struct{}),
		CloudBindAddr:      "0.0.0.0:54321",
		HTTPBindAddr:       "0.0.0.0:54320",
		cloudEndpoints:     make(map[string]struct{}),
		cloudSessions:      make(map[int]chan []byte),
		cloudKeepAliveCh:   make(chan net.Addr, 16),
		stopCh:             make(chan struct{}),
		logger: zerolog.New(os.Stdout).With().
			Str("app", "rocklet").
			Logger().Level(zerolog.DebugLevel),

		incomingQueues:        make(map[string]chan *Method),
		outgoingQueueInternal: make(chan *Method, 0),
		outgoingQueueAppProxy: make(chan *Method, 0),
		rand:  rand.New(rand.NewSource(time.Now().UnixNano())),
		dir:   "/mnt/data/miio/",
		flags: flags,
	}

	// initialize metrics
	r.metrics = metrics.New(r.logger)

	// initialize navmap
	r.navMap = navmap.New(flags)

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
	localAddr, err := net.ResolveUDPAddr("udp", r.CloudBindAddr)
	if err != nil {
		return fmt.Errorf("error resolving cloud bind address: %s", err)
	}

	r.cloudConnection, err = net.ListenUDP("udp", localAddr)
	if err != nil {
		return fmt.Errorf("error binding cloud socket: %s", err)
	}
	r.Logger().Debug().Msgf("successfully bound cloud server to %s", r.CloudBindAddr)
	// handle closing
	go func() {
		<-r.stopCh
		if err := r.cloudConnection.Close(); err != nil {
			r.logger.Warn().Err(err).Msg("error closing cloud connection")
		}
		r.Logger().Debug().Msgf("closed cloud server at %s", r.CloudBindAddr)
	}()

	// listen for local connections
	r.localListener, err = net.Listen("tcp", r.LocalBindAddr)
	if err != nil {
		return fmt.Errorf("error binding local socket: %s", err)
	}
	r.Logger().Debug().Msgf("successfully bound local server to %s", r.LocalBindAddr)
	// handle closing
	go func() {
		<-r.stopCh
		if err := r.localListener.Close(); err != nil {
			r.logger.Warn().Err(err).Msg("error closing local listener")
		}
		r.Logger().Debug().Msgf("closed local server at %s", r.LocalBindAddr)
	}()

	// listen for http connections
	r.httpListener, err = net.Listen("tcp", r.HTTPBindAddr)
	if err != nil {
		return fmt.Errorf("error binding http socket: %s", err)
	}
	r.Logger().Debug().Msgf("successfully bound http server to %s", r.HTTPBindAddr)
	// handle closing
	go func() {
		<-r.stopCh
		if err := r.httpListener.Close(); err != nil {
			r.logger.Warn().Err(err).Msg("error closing http listener")
		}
		r.Logger().Debug().Msgf("closed http server at %s", r.LocalBindAddr)
	}()

	return nil
}

func (r *Rockrobo) GetInternalInfo() (*MethodParamsResponseInternalInfo, error) {
	// retrieve device ID
	response, err := r.retrieve(
		&Method{
			Method: MethodInternalInfo,
		},
		MethodInternalInfo,
	)
	if err != nil {
		return nil, err
	}

	var params MethodParamsResponseInternalInfo
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

func (r *Rockrobo) retrieve(requestObj *Method, methodResponse string) (*Method, error) {
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
	r.outgoingQueueInternal <- requestObj

	// wait for response
	// TODO add timeout
	resp := <-dataCh

	r.Logger().Debug().Interface("resp", resp).Msg("data received")

	return resp, nil
}

// handle sigterm sigint
func (r *Rockrobo) runSignalHandling(sigs chan os.Signal) {
	for sig := range sigs {
		if r.stopCh != nil {
			close(r.stopCh)
			r.stopCh = nil
		}
		r.Logger().Info().Msgf("received signal: %s", sig.String())
	}
}

// http server for navigation maps and metrics
func (r *Rockrobo) runHTTPServer() {
	defer r.waitGroup.Done()

	// create mux
	serveMux := http.NewServeMux()

	// setup navmaps
	r.navMap.SetupHandler(serveMux)

	// setup metrics
	serveMux.Handle("/metrics", r.metrics.Handler())
	r.metrics.BatteryLevel.WithLabelValues("test").Set(91.11)

	// serve http
	if err := http.Serve(r.httpListener, serveMux); err != nil {
		if x, ok := err.(*net.OpError); ok && x.Err.Error() == "use of closed network connection" {
			return
		}
		r.logger.Warn().Err(err).Msg("error serving http server")
	}

	r.logger.Debug().Msg("stopping http server")
}

func (r *Rockrobo) Run() error {
	// setup signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go r.runSignalHandling(sigs)

	// bind sockets
	err := r.Listen()
	if err != nil {
		return err
	}

	// listen to rpc on unix socket
	if err := r.runRPCServer(); err != nil {
		return err
	}

	// run http server for metrics/navmaps
	r.waitGroup.Add(1)
	go r.runHTTPServer()

	// run navmap loop
	go func() {
		defer r.waitGroup.Done()
		r.navMap.LoopMaps(r.stopCh)
	}()

	// test sqlite3 list
	cleanings, err := r.navMap.ListCleanings(r.flags.RobotDatabase)
	if err != nil {
		return err
	}
	r.logger.Info().Msgf("%d cleanings found", len(cleanings))

	// run local tcp connection handler
	r.waitGroup.Add(1)
	go r.runLocalConnectionHandler()

	// run local device init
	go r.runLocalDeviceInit()

	// start udp receiver
	r.waitGroup.Add(1)
	go r.runCloudReceiver()

	// keep alive cloud connection
	go r.runCloudKeepAlive()

	// wait for exist signal
	<-r.stopCh

	r.waitGroup.Wait()

	return nil
}
