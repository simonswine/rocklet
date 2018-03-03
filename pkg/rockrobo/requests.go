package rockrobo

// request the device id
const RequestDeviceIDMethod = "_internal.request_dinfo"

type RequestDeviceID struct {
	Method string `json:"method"`
	Params string `json:"params"`
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
