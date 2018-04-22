package rockrobo

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

const (
	MethodHello                    = "_internal.hello"
	MethodRequestDeviceID          = "_internal.request_dinfo"
	MethodResponseDeviceID         = "_internal.response_dinfo"
	MethodRequestToken             = "_internal.request_dtoken"
	MethodResponseToken            = "_internal.response_dtoken"
	MethodInternalInfo             = "_internal.info"
	MethodRequestWifiConfigStatus  = "_internal.req_wifi_conf_status"
	MethodResponseWifiConfigStatus = "_internal.res_wifi_conf_status"

	MethodOTCInfo = "_otc.info"

	MethodLocalStatus                  = "local.status"
	MethodLocalStatusInternetConnected = "internet_connected"
	MethodLocalStatusCloudConnected    = "cloud_connected"

	MethodLocalAppRCStart = "app_rc_start"
	MethodLocalAppRCEnd   = "app_rc_end"
	MethodLocalAppRCMove  = "app_rc_move"

	MethodLocalAppStart  = "app_rc_start"
	MethodLocalAppStop   = "app_rc_end"
	MethodLocalAppPause  = "app_pause"
	MethodLocalAppSpot   = "app_spot"
	MethodLocalAppCharge = "app_charge"
)

type Method struct {
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	ID        int             `json:"id,omitempty"`
	PartnerID string          `json:"partner_id,omitempty"`
}

type MethodParamsResponseDeviceID struct {
	DeviceID int    `json:"did"`
	Key      []byte `json:"key"`
	Vendor   string `json:"vendor"`
	MAC      string `json:"mac"`
	Model    string `json:"model"`
}

type MethodParamsRequestToken struct {
	Dir    string `json:"dir"`
	NToken []byte `json:"ntoken"`
}

type MethodParamsRequestAppRC struct {
	Velocity       *float64 `json:"velocity,omitempty"`
	Omega          *float64 `json:"omega,omitempty"`
	Duration       *int     `json:"duration,omitempty"`
	SequenceNumber int      `json:"seqnum,omitempty"`
}

type MethodParamsResponseInternalInfo struct {
	HardwareVersion string `json:"hw_ver"`
	FirmwareVersion string `json:"fw_ver"`
	Model           string `json:"model"`
	MAC             string `json:"mac"`
	Token           string `json:"token"`
	Life            int    `json:"life"`

	AccessPoint      MethodParamsResponseInternalInfoAccessPoint      `json:"ap"`
	NetworkInterface MethodParamsResponseInternalInfoNetworkInterface `json:"netif"`
}

type MethodParamsResponseInternalInfoAccessPoint struct {
	SSID  string `json:"ssid"`
	BSSID string `json:"bssid"`
	RSSI  int    `json:"rssi"`
}

type MethodParamsResponseInternalInfoNetworkInterface struct {
	LocalIP string `json:"localIp"`
	Mask    string `json:"mask"`
	Gateway string `json:"gw"`
}

type IPPort struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type MethodResultOTCInfo struct {
	OTCList []IPPort            `json:"otc_list"`
	OTCTest MethodResultOTCTest `json:"otc_test"`
}

type MethodResultOTCTest struct {
	List      []IPPort `json:"list"`
	FirstTest int      `json:"firsttest"`
	Interval  int      `json:"interval"`
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
		log := c.logger

		for {
			var method Method
			err := d.Decode(&method)

			if err == io.EOF {
				log.Debug().Msg("received EOF")
				return
			} else if err != nil {
				c.logger.Warn().Err(err).Msg("error reading local connection")
				return
			}
			log.Debug().Interface("data", method).Msg("received data on local connection")

			// if hello received for the frist time, signal readiness
			if method.Method == MethodHello {
				if !readySent {
					close(readyCh)
					log.Debug().Msg("readyCh now closed")
					readySent = true
				}
				continue
			}

			channelName := method.Method
			if !readySent && !strings.HasPrefix(method.Method, "_internal.") {
				channelName = "cloud"
			}

			// retrieve correct channel
			c.rockrobo.incomingQueuesLock.Lock()
			dataCh, ok := c.rockrobo.incomingQueues[channelName]
			c.rockrobo.incomingQueuesLock.Unlock()

			if !ok {
				log.Warn().Interface("channel", channelName).Msg("no channel to handle this method")
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

		var sendChannel *chan *Method
		log := c.logger

		// wait for ready or connection close
		select {
		case <-readyCh:
			sendChannel = &c.rockrobo.outgoingQueueInternal
			log = log.With().Str("connection", "internal").Logger()
			log.Debug().Msg("local connection ready")
		case <-time.After(1 * time.Second):
			sendChannel = &c.rockrobo.outgoingQueueAppProxy
			log = log.With().Str("connection", "appproxy").Logger()
			log.Debug().Msg("local connection ready")
		case <-c.closeCh:
			log.Debug().Msg("break outgoing as connection closed")
			return
		}

		e := json.NewEncoder(c.conn)

		for {
			select {
			case obj := <-*sendChannel:
				err := e.Encode(obj)
				if err != nil {
					log.Warn().Err(err).Interface("data", obj).Msg("error sending data")
				}
				log.Debug().Interface("data", obj).Msg("send data")

			case <-c.closeCh:
				log.Debug().Msg("break outgoing as connection closed")
				return
			}
		}

	}()

	wg.Wait()

}

// discover local device ID
func (r *Rockrobo) LocalDeviceID() (*MethodParamsResponseDeviceID, error) {
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

// get the wifi status from local
func (r *Rockrobo) LocalWifiConfigStatus() (int, error) {
	// retrieve device ID
	response, err := r.retrieve(
		&Method{
			Method: MethodRequestWifiConfigStatus,
			Params: json.RawMessage(fmt.Sprintf(`"%s"`, r.dir)),
		},
		MethodResponseWifiConfigStatus,
	)
	if err != nil {
		return -1, err
	}

	var params int
	err = json.Unmarshal(response.Params, &params)
	if err != nil {
		return -1, err
	}

	return params, nil

}

// get the status from local
func (r *Rockrobo) LocalSetStatus(status string) error {
	data, err := json.Marshal(&status)
	if err != nil {
		return err
	}

	r.outgoingQueueAppProxy <- &Method{
		Method: MethodLocalStatus,
		Params: json.RawMessage(data),
	}

	return nil
}

// start remote control mode
func (r *Rockrobo) LocalAppRCStart() error {
	r.appRCSequenceNumber = 1
	r.outgoingQueueAppProxy <- &Method{
		Method: MethodLocalAppRCStart,
		ID:     r.rand.Int(),
	}
	return nil
}

// end remote control mode
func (r *Rockrobo) LocalAppRCEnd() error {
	params := []MethodParamsRequestAppRC{
		{
			SequenceNumber: r.appRCSequenceNumber,
		},
	}
	paramsData, err := json.Marshal(&params)
	if err != nil {
		return err
	}
	r.appRCSequenceNumber++

	r.outgoingQueueAppProxy <- &Method{
		Method: MethodLocalAppRCEnd,
		ID:     r.rand.Int(),
		Params: json.RawMessage(paramsData),
	}

	return nil
}

// do a remote control move
func (r *Rockrobo) LocalAppRCMove(velocity, omega float64, duration int) error {
	params := []MethodParamsRequestAppRC{
		{
			SequenceNumber: r.appRCSequenceNumber,
			Duration:       &duration,
			Omega:          &omega,
			Velocity:       &velocity,
		},
	}
	paramsData, err := json.Marshal(&params)
	if err != nil {
		return err
	}
	r.appRCSequenceNumber++

	r.outgoingQueueAppProxy <- &Method{
		Method: MethodLocalAppRCMove,
		ID:     r.rand.Int(),
		Params: json.RawMessage(paramsData),
	}

	return nil
}

// start cleaning
func (r *Rockrobo) LocalAppStart() error {
	r.outgoingQueueAppProxy <- &Method{
		Method: MethodLocalAppStart,
		ID:     r.rand.Int(),
	}
	return nil
}

func (r *Rockrobo) LocalAppStop() error {
	r.outgoingQueueAppProxy <- &Method{
		Method: MethodLocalAppStop,
		ID:     r.rand.Int(),
	}
	return nil
}

func (r *Rockrobo) LocalAppPause() error {
	r.outgoingQueueAppProxy <- &Method{
		Method: MethodLocalAppPause,
		ID:     r.rand.Int(),
	}
	return nil
}

func (r *Rockrobo) LocalAppSpot() error {
	r.outgoingQueueAppProxy <- &Method{
		Method: MethodLocalAppSpot,
		ID:     r.rand.Int(),
	}
	return nil
}

func (r *Rockrobo) LocalAppCharge() error {
	r.outgoingQueueAppProxy <- &Method{
		Method: MethodLocalAppCharge,
		ID:     r.rand.Int(),
	}
	return nil
}

// local connection handler
func (r *Rockrobo) runLocalConnectionHandler() {
	defer r.waitGroup.Done()
	for {
		//accept connections using Listener.Accept()
		conn, err := r.localListener.Accept()
		if err != nil {
			if x, ok := err.(*net.OpError); ok && x.Op == "accept" { // We're done
				break
			}
			r.logger.Warn().Err(err).Msg("accepting local connection failed")
			continue
		}
		//It's common to handle accepted connection on different goroutines
		c := r.newLocalConnection(conn)
		go c.handle()
	}
	r.logger.Debug().Msg("stopping local connection handler")
}

// init local device
func (r *Rockrobo) runLocalDeviceInit() {
	var err error

	for {
		r.logger.Info().Msg("waiting for device to connect")
		// get device ID
		r.deviceID, err = r.LocalDeviceID()
		if err != nil {
			r.logger.Error().Err(err).Msg("failed to get local device ID")
			continue
		}

		// get device token
		r.dToken, err = r.GetToken()
		if err != nil {
			r.logger.Error().Err(err).Msg("failed to get local device token")
			continue
		}

		// get wifi config status
		wifiConfigState, err := r.LocalWifiConfigStatus()
		if err != nil {
			r.logger.Error().Err(err).Msg("failed to get local wifi config status")
			continue
		}
		r.logger.Debug().Err(err).Int("wifi_config_state", wifiConfigState).Msg("retrieved wifi config state")

		r.logger.Info().Int("device_id", r.deviceID.DeviceID).Msg("device ready")
		close(r.localDeviceReadyCh)
		break
	}
}
