package rockrobo

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/simonswine/rocklet/pkg/api"
)

const CloudDNSName = "ot.io.mi.com"

type Rockrobo struct {
	CloudBindAddr     string
	cloudConnection   *net.UDPConn
	cloudEndpoints    map[string]struct{} // this is a maintained map of accessbile cloud endpoints
	cloudSessions     map[int]chan []byte
	cloudSessionsLock sync.Mutex

	LocalBindAddr string
	localListener net.Listener

	stopCh chan struct{}

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
}

func New(flags *api.Flags) *Rockrobo {
	r := &Rockrobo{
		LocalBindAddr:  "127.0.0.1:54322",
		CloudBindAddr:  "0.0.0.0:54321",
		cloudEndpoints: make(map[string]struct{}),
		cloudSessions:  make(map[int]chan []byte),
		logger: zerolog.New(os.Stdout).With().
			Str("app", "rocklet").
			Logger().Level(zerolog.DebugLevel),

		incomingQueues:        make(map[string]chan *Method),
		outgoingQueueInternal: make(chan *Method, 0),
		outgoingQueueAppProxy: make(chan *Method, 0),
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
		dir:  "/mnt/data/miio/",
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

	// listen for local connections
	r.localListener, err = net.Listen("tcp", r.LocalBindAddr)
	if err != nil {
		return fmt.Errorf("error binding local socket: %s", err)
	}
	r.Logger().Debug().Msgf("successfully bound local server to %s", r.LocalBindAddr)

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

	// get device ID
	r.deviceID, err = r.LocalDeviceID()
	if err != nil {
		return err
	}

	// get device token
	r.dToken, err = r.GetToken()
	if err != nil {
		return err
	}

	// get wifi config status
	wifiConfigState, err := r.LocalWifiConfigStatus()
	if err != nil {
		return err
	}
	r.logger.Debug().Err(err).Int("wifi_config_state", wifiConfigState).Msg("retrieved wifi config state")

	// start udp receiver
	keepAliveCh := make(chan net.Addr, 16)
	go func() {
		for {
			packet := make([]byte, 4096)
			length, raddr, err := r.cloudConnection.ReadFrom(packet)
			if err != nil {
				r.logger.Warn().Err(err).Msg("failed cloud connection read")
				continue
			}

			reader := bytes.NewReader(packet[0:length])

			var msg CloudMessage

			err = msg.Read(reader, []byte(base64.StdEncoding.EncodeToString(r.deviceID.Key)))
			if err != nil {
				r.logger.Warn().Str("remote_addr", raddr.String()).Err(err).Msg("failed to decode cloud message")
			}

			if msg.Length == uint16(32) {
				keepAliveCh <- raddr
				continue
			}

			// parse payload
			var method Method
			if err := json.Unmarshal(msg.Body, &method); err != nil {
				r.logger.Warn().Str("remote_addr", raddr.String()).Str("payload", string(msg.Body)).Err(err).Msg("unable to parse payload")
			}

			r.cloudSessionsLock.Lock()
			ch, ok := r.cloudSessions[method.ID]
			if ok {
				delete(r.cloudSessions, method.ID)
			}
			r.cloudSessionsLock.Unlock()

			if ok {
				ch <- msg.Body
			} else {
				r.logger.Warn().Str("remote_addr", raddr.String()).Interface("method", method).Msg("unhandled message from cloud")
			}

		}
	}()

	// keep alive cloud connection
	go func() {
	ConnectionLoop:
		for {

			// remove existing cloud forwarder
			r.incomingQueuesLock.Lock()
			delete(r.incomingQueues, "cloud")
			r.incomingQueuesLock.Unlock()
			var incomingQueueCloudCh *chan *Method

			// look up cloud endpoints if none are available
			if len(r.cloudEndpoints) == 0 {
				hosts, err := net.LookupHost(CloudDNSName)
				if err != nil {
					r.logger.Warn().Err(err).Msgf("failed to resolve '%s'", CloudDNSName)
					continue
				}
				for _, host := range hosts {
					r.cloudEndpoints[host] = struct{}{}
				}
			}

			// skip if no cloud endpoint available
			if len(r.cloudEndpoints) == 0 {
				r.logger.Warn().Err(err).Msg("no cloud endpoints available")
				continue
			}

			// select an endpoint randomly
			i := r.rand.Intn(len(r.cloudEndpoints))
			var host string
			for host = range r.cloudEndpoints {
				if i == 0 {
					break
				}
				i--
			}
			remoteAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:8053", host))
			if err != nil {
				r.logger.Warn().Err(err).Msg("error getting UDP addr")
				continue
			}

			var failedAttemps int
			var nextStatus time.Time

		SessionLoop:
			for {

				// get rid of this endpoint, too many failed attemps
				if failedAttemps != 0 {
					time.Sleep(1 * time.Second)
					if failedAttemps > 5 {
						r.logger.Warn().Str("remote_addr", remoteAddr.String()).Msgf("removing endpoint after %d keep alive failed attemps", failedAttemps)
						delete(r.cloudEndpoints, host)
						continue ConnectionLoop
					}
				}

				// build hello packet
				buf := new(bytes.Buffer)
				err = NewHelloCloudMessage().Write(buf, []byte(base64.StdEncoding.EncodeToString(r.deviceID.Key)))
				if err != nil {
					failedAttemps++
					continue
				}

				_, err = r.cloudConnection.WriteToUDP(buf.Bytes(), remoteAddr)
				if err != nil {
					failedAttemps++
					continue
				}
				r.logger.Debug().Str("remote_addr", remoteAddr.String()).Msg("sent hello cloud message")

				select {
				case helloAddr := <-keepAliveCh:
					if helloAddr.String() == remoteAddr.String() {
						break
					} else {
						r.logger.Warn().Str("remote_addr", remoteAddr.String()).Msgf("response to hello cloud message received from wrong address '%s'", helloAddr)
						continue
					}
				case <-time.After(5 * time.Second):
					r.logger.Debug().Str("remote_addr", remoteAddr.String()).Msg("response to hello cloud message timed out")
					failedAttemps++
					continue SessionLoop
				}
				r.logger.Debug().Str("remote_addr", remoteAddr.String()).Msg("received response to hello cloud message")

				if incomingQueueCloudCh == nil {
					cloudCh := make(chan *Method)
					r.incomingQueuesLock.Lock()
					r.incomingQueues["cloud"] = cloudCh
					r.incomingQueuesLock.Unlock()
					incomingQueueCloudCh = &cloudCh
				}

				// report status, if new connection
				if nextStatus.IsZero() {
					r.LocalSetStatus(MethodLocalStatusInternetConnected)
					time.Sleep(time.Second)
				}

				// send internal info upstream
				if nextStatus.Before(time.Now()) {
					otcInfo, err := r.CloudOTCInfo(remoteAddr)
					if err != nil {
						r.logger.Warn().Str("remote_addr", remoteAddr.String()).Err(err).Msg("failed to report otc info")
						failedAttemps++
						continue SessionLoop
					}

					// schedule next run of status upload
					nextStatus = time.Now().Add(time.Second * time.Duration(otcInfo.OTCTest.Interval))

					// add hosts to endpoint list
					for _, host := range otcInfo.OTCList {
						r.cloudEndpoints[host.IP] = struct{}{}
					}

					// report status
					r.LocalSetStatus(MethodLocalStatusCloudConnected)

				}

				failedAttemps = 0

				select {
				case m := <-*incomingQueueCloudCh:
					mBytes, err := json.Marshal(m)
					if err != nil {
						r.logger.Warn().Err(err).Msg("error marshalling json cloud message")
						continue
					}

					msg := NewCloudMessage()
					msg.Body = mBytes

					// build cloud message
					buf := new(bytes.Buffer)
					if err := msg.Write(buf, []byte(base64.StdEncoding.EncodeToString(r.deviceID.Key))); err != nil {
						r.logger.Warn().Msg("error creating cloud message")
						continue
					}

					_, err = r.cloudConnection.WriteToUDP(buf.Bytes(), remoteAddr)
					if err != nil {
						failedAttemps++
						continue
					}
					r.logger.Debug().Str("remote_addr", remoteAddr.String()).Msg("sent message")
				case <-time.After(15 * time.Second):
					continue
				}
			}
		}
	}()

	// wait for exist signal
	<-r.stopCh

	r.waitGroup.Wait()

	return nil
}

func (r *Rockrobo) cloudRegisterID(id int, dataCh chan []byte) {
	r.cloudSessionsLock.Lock()
	r.cloudSessions[id] = dataCh
	r.cloudSessionsLock.Unlock()
}
