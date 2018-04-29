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
	"github.com/simonswine/rocklet/pkg/apis/vacuum/v1alpha1"
	"github.com/simonswine/rocklet/pkg/kubernetes"
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

	// kubernetes module
	kubernetes     *kubernetes.Kubernetes
	cleaningCh     chan *v1alpha1.Cleaning
	vacuumStatusCh chan v1alpha1.VacuumStatus
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

	// initialize kubernetes
	if r.flags.Kubernetes.Enabled {
		r.cleaningCh = make(chan *v1alpha1.Cleaning, 16)
		r.vacuumStatusCh = make(chan v1alpha1.VacuumStatus)
		r.kubernetes = kubernetes.NewInternal(flags, r.stopCh, r.vacuumStatusCh, r.cleaningCh)
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
	response, err := r.retrieveInternal(
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
	response, err := r.retrieveInternal(
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

func (r *Rockrobo) retrieve(requestObj *Method, methodResponse string, outgoing chan *Method) (*Method, error) {
	dataCh := make(chan *Method)

	repsonseHandled := false
	// handle id based call back
	if requestObj.ID != 0 {
		idKey := fmt.Sprintf("id=%d", requestObj.ID)
		r.incomingQueuesLock.Lock()
		r.incomingQueues[idKey] = dataCh
		r.incomingQueuesLock.Unlock()
		defer func() {
			r.incomingQueuesLock.Lock()
			delete(r.incomingQueues, idKey)
			r.incomingQueuesLock.Unlock()
		}()
		repsonseHandled = true
	}

	// handle method resposne callback
	if methodResponse != "" {
		r.incomingQueuesLock.Lock()
		r.incomingQueues[methodResponse] = dataCh
		r.incomingQueuesLock.Unlock()
		defer func() {
			r.incomingQueuesLock.Lock()
			delete(r.incomingQueues, methodResponse)
			r.incomingQueuesLock.Unlock()
		}()
		repsonseHandled = true
	}

	if !repsonseHandled {
		return nil, fmt.Errorf("no callback setup for this method: %+v", requestObj)

	}

	// write message
	outgoing <- requestObj

	// wait for response
	// TODO add timeout
	resp := <-dataCh

	r.Logger().Debug().Interface("resp", resp).Msg("data received")

	return resp, nil
}

func (r *Rockrobo) retrieveAppProxy(requestObj *Method) (*Method, error) {
	if requestObj.ID == 0 {
		requestObj.ID = r.rand.Int()
	}
	return r.retrieve(requestObj, "", r.outgoingQueueAppProxy)
}

func (r *Rockrobo) retrieveInternal(requestObj *Method, methodResponse string) (*Method, error) {
	return r.retrieve(requestObj, methodResponse, r.outgoingQueueInternal)
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

	// serve http
	if err := http.Serve(r.httpListener, serveMux); err != nil {
		if x, ok := err.(*net.OpError); ok && x.Err.Error() == "use of closed network connection" {
			return
		}
		r.logger.Warn().Err(err).Msg("error serving http server")
	}

	r.logger.Debug().Msg("stopping http server")
}

func (r *Rockrobo) CloudStart() {
	// start udp receiver
	r.waitGroup.Add(1)
	go r.runCloudReceiver()

	// keep alive cloud connection
	go r.runCloudKeepAlive()
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

	// run kubernetes integration
	if r.flags.Kubernetes.Enabled {
		go r.kubernetes.Run()

		// initial read of cleanings
		go func() {
			cleanings, err := r.navMap.ListCleanings()
			if err != nil {
				r.logger.Error().Err(err).Msg("error reading historic cleanings")
				return
			}
			r.logger.Info().Msgf("%d cleanings found", len(cleanings))
			for _, c := range cleanings {
				r.cleaningCh <- c
			}
		}()

		// send status updates every 15 seconds
		go func() {
			// wait for device ready
			<-r.localDeviceReadyCh

			tickerStatus := time.NewTicker(2 * time.Second).C
			tickerInternalInfo := time.NewTicker(60 * time.Second).C

			var internalInfo *MethodParamsResponseInternalInfo
			getInternalInfo := func() {
				var err error
				internalInfo, err = r.GetInternalInfo()
				if err != nil {
					r.logger.Warn().Err(err).Msg("cannot get internal info")
				}
			}

			updateStatus := func() {
				status, err := r.LocalGetStatus()
				if err != nil {
					r.logger.Warn().Err(err).Msg("error getting status from device")
				}

				state, ok := v1alpha1.VacuumStateHash[status.State]
				if !ok {
					r.logger.Warn().Int("state", status.State).Msg("cannot map state with known status")
					state = v1alpha1.VacuumStateUnknown
				}

				vacuumStatus := &v1alpha1.VacuumStatus{
					Area:         status.CleanArea,
					Duration:     (time.Second * time.Duration(status.CleanTime)).String(),
					BatteryLevel: status.Battery,
					FanPower:     status.FanPower,
					DoNotDisturb: status.DNDEnabled == 1,
					State:        state,
					ErrorCode:    status.ErrorCode,
				}

				r.metrics.BatteryLevel.WithLabelValues(r.flags.Kubernetes.NodeName).Set(float64(status.Battery))

				if r.deviceID != nil {
					vacuumStatus.MAC = r.deviceID.MAC
					vacuumStatus.DeviceID = r.deviceID.DeviceID
				}

				if internalInfo != nil {
					vacuumStatus.WifiSSID = internalInfo.AccessPoint.SSID
					vacuumStatus.WifiRSSI = internalInfo.AccessPoint.RSSI
				}

				// get map if available
				if status.MapPresent != 0 {
					p, m, c, err := r.navMap.LatestPositionsMap()
					if err != nil {
						r.logger.Warn().Err(err).Msg("error getting local map")
					} else {
						vacuumStatus.Map = m
						vacuumStatus.Path = p
						if c != nil {
							vacuumStatus.Charger = c
						}
					}
				}

				if err := r.kubernetes.UpdateVaccumStatus(vacuumStatus); err != nil {
					r.logger.Warn().Interface("status", vacuumStatus).Err(err).Msg("failed updating vacuum status")
				}

			}

			getInternalInfo()
			updateStatus()

			for {
				select {
				case <-r.stopCh:
					return
				case <-tickerInternalInfo:
					getInternalInfo()
				case <-tickerStatus:
					updateStatus()
				}
			}
		}()
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

	// run local tcp connection handler
	r.waitGroup.Add(1)
	go r.runLocalConnectionHandler()

	// run local device init
	go r.runLocalDeviceInit()

	if r.flags.Cloud.Enabled {
		r.CloudStart()
	}

	// wait for exist signal
	<-r.stopCh

	r.waitGroup.Wait()

	return nil
}
