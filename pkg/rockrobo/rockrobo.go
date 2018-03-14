package rockrobo

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
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

type Rockrobo struct {
	GlobalBindAddr   string
	globalConnection *net.UDPConn

	LocalBindAddr string
	localListener net.Listener

	stopCh chan struct{}

	logger zerolog.Logger

	outgoingQueue      chan interface{}
	incomingQueues     map[string]chan *Method
	incomingQueuesLock sync.Mutex

	deviceID     *MethodParamsResponseDeviceID
	internalInfo *MethodParamsResponseInternalInfo

	rand   *rand.Rand
	nToken []byte
	dToken []byte
	dir    string
}

func New(flags *api.Flags) *Rockrobo {
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
	localAddr, err := net.ResolveUDPAddr("udp", r.GlobalBindAddr)
	if err != nil {
		return fmt.Errorf("error resolving global bind address: %s", err)
	}

	r.globalConnection, err = net.ListenUDP("udp", localAddr)
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

	// get device ID
	r.deviceID, err = r.GetDeviceID()
	if err != nil {
		return err
	}

	// get device token
	r.dToken, err = r.GetToken()
	if err != nil {
		return err
	}

	// get internal info
	r.internalInfo, err = r.GetInternalInfo()
	if err != nil {
		return err
	}

	// start udp receiver
	go func() {
		for {
			packet := make([]byte, 4096)
			length, raddr, err := r.globalConnection.ReadFrom(packet)
			if err != nil {
				r.logger.Warn().Err(err).Msg("failed global connection read")
				continue
			}

			reader := bytes.NewReader(packet[0:length])

			var msg CloudMessage
			err = msg.Read(reader, []byte(base64.StdEncoding.EncodeToString(r.deviceID.Key)))
			if err != nil {
				r.logger.Warn().Str("remote_addr", raddr.String()).Err(err).Msg("failed to decode cloud message")
			}

			r.logger.Debug().Str("remote_addr", raddr.String()).Interface("message", msg).Str("packet", fmt.Sprintf("%x", packet[0:length])).Msg("received cloud message")
		}
	}()

	// build hello packet
	buf := new(bytes.Buffer)
	err = NewHelloCloudMessage().Write(buf, []byte{})
	if err != nil {
		return err
	}

	// resolve cloud destination
	remoteAddr, err := net.ResolveUDPAddr("udp", "ot.io.mi.com:8053")
	if err != nil {
		return err
	}

	_, err = r.globalConnection.WriteToUDP(buf.Bytes(), remoteAddr)
	if err != nil {
		return err
	}
	r.logger.Debug().Str("remote_addr", remoteAddr.String()).Msg("sent hello cloud message")

	// set token, which is device token in base64 and hex
	r.internalInfo.Token = hex.EncodeToString([]byte(base64.StdEncoding.EncodeToString(r.dToken)))

	r.internalInfo.MAC = r.deviceID.MAC
	r.internalInfo.Model = r.deviceID.Model
	r.internalInfo.Life = 1637

	// write json params
	paramsOtcInfo, err := json.Marshal(r.internalInfo)
	if err != nil {
		return err
	}

	method := &Method{
		ID:     923147530,
		Params: json.RawMessage(paramsOtcInfo),
		Method: MethodOTCInfo,
	}

	messageBody, err := json.Marshal(method)
	if err != nil {
		return err
	}
	messageString := string(messageBody)

	buf = new(bytes.Buffer)
	message := NewCloudMessage()
	message.DeviceID = uint32(r.deviceID.DeviceID)
	message.Body = messageBody
	err = message.Write(buf, []byte{})
	if err != nil {
		return err
	}

	time.Sleep(time.Second)
	_, err = r.globalConnection.WriteToUDP(buf.Bytes(), remoteAddr)
	if err != nil {
		return err
	}
	r.logger.Debug().Str("remote_addr", remoteAddr.String()).Str("message", string(messageString)).Msg("sent init cloud message")

	// wait for exist signal
	<-r.stopCh

	return nil
}
