package rockrobo

import (
	"encoding/json"
	"net"
)

// request hello
const RequestHelloMethod = "_internal.request_dinfo"

type RequestHello struct {
	Method string `json:"method"`
}

func requestHello(conn net.Conn) error {
	data, err := json.Marshal(&RequestDeviceID{
		Method: RequestDeviceIDMethod,
		Params: "/etc/miio/",
	})
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err

}

// request the device id
const RequestDeviceIDMethod = "_internal.request_dinfo"

type RequestDeviceID struct {
	Method string `json:"method"`
	Params string `json:"params"`
}

func requestDeviceID(conn net.Conn) error {
	data, err := json.Marshal(&RequestDeviceID{
		Method: RequestDeviceIDMethod,
		Params: "/mnt/data/miio/",
	})
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err

}

// request the token
const RequestTokenMethod = "_internal.request_dtoken"

type RequestToken struct {
	Method string              `json:"method"`
	Params *RequestTokenParams `json:"params"`
}

type RequestTokenParams struct {
	Dir    string `json:"dir"`
	NToken string `json:"ntoken"`
}
