package rockrobo

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"reflect"
	"time"
)

var CloudMessageHeaderMagic = [2]byte{0x21, 0x31}

type CloudMessageHeader struct {
	Header   [2]byte
	Length   uint16
	Unknown  [4]byte
	DeviceID uint32
	Epoch    uint32
	Checksum [16]byte
}

type CloudMessage struct {
	CloudMessageHeader

	Body []byte
}

func NewHelloCloudMessage() *CloudMessage {
	m := NewCloudMessage()
	m.DeviceID = 0xffffffff
	m.Unknown = [4]byte{0xff, 0xff, 0xff, 0xff}
	return m
}

func NewCloudMessage() *CloudMessage {
	return &CloudMessage{
		CloudMessageHeader: CloudMessageHeader{
			Epoch:  uint32(time.Now().UTC().Unix()),
			Header: CloudMessageHeaderMagic,
		},
	}
}

func (m *CloudMessage) Read(reader io.Reader, key []byte) error {
	err := binary.Read(reader, binary.BigEndian, &m.CloudMessageHeader)
	if err != nil {
		return fmt.Errorf("error while reading header: %s", err)
	}

	m.Body, err = ioutil.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("error while reading body: %s", err)
	}

	// verify size
	if act, exp := uint16(binary.Size(&m.CloudMessageHeader)+len(m.Body)), m.Length; act != exp {
		return fmt.Errorf("error size of header and actual size mismatch: header=%d actual=%d", exp, act)
	}

	// if header only don't verify checksums
	if int(m.Length) == binary.Size(&m.CloudMessageHeader) {
		return nil
	}

	// verify checksum is matching
	checksumExp, err := m.Checksum(key)
	if err != nil {
		return fmt.Errorf("error building the checksum: %s", err)
	}
	if !reflect.DeepEqual(m.CloudMessageHeader.Checksum, checksumExp) {
		return fmt.Errorf("checksum's do not match, actual=%x calculated=%x", m.CloudMessageHeader.Checksum, checksumExp)
	}

	// decrypt
	aesKey, aesIV := aesParamsFromKey(key)
	aesCipher, err := aes.NewCipher(aesKey)
	if err != nil {
		return err
	}

	mode := cipher.NewCBCDecrypter(aesCipher, aesIV)
	mode.CryptBlocks(m.Body, m.Body)

	// unpad plaintext
	m.Body, err = unpad(m.Body)
	if err != nil {
		return err
	}

	// remove null termination
	m.Body = bytes.Trim(m.Body, "\x00")

	return nil
}

func (m *CloudMessage) Write(writer io.Writer, key []byte) (err error) {
	// copy into temporary message
	mNew := *m

	// encrypt if there is a body
	if len(m.Body) > 0 {
		aesKey, aesIV := aesParamsFromKey(key)
		aesCipher, err := aes.NewCipher(aesKey)
		if err != nil {
			return err
		}

		// pad plaintext
		mNew.Body = pad(m.Body)

		mode := cipher.NewCBCEncrypter(aesCipher, aesIV)
		mode.CryptBlocks(mNew.Body, mNew.Body)
	}

	// set length of package
	mNew.Length = uint16(binary.Size(&m.CloudMessageHeader) + len(mNew.Body))

	// set checksum if there is a body
	if len(m.Body) > 0 {
		mNew.CloudMessageHeader.Checksum, err = mNew.Checksum(key)
		if err != nil {
			return err
		}
	}

	// write header
	err = binary.Write(writer, binary.BigEndian, &mNew.CloudMessageHeader)
	if err != nil {
		return err
	}

	// write body
	_, err = writer.Write(mNew.Body)
	if err != nil {
		return err
	}

	return nil

}

func aesParamsFromKey(key []byte) (awsKey []byte, aesIV []byte) {
	// initialize aes values
	h := md5.New()
	h.Write(key)
	aesKey := h.Sum(nil)

	h.Reset()
	h.Write(aesKey)
	h.Write(key)

	aesIV = h.Sum(nil)

	return aesKey, aesIV
}

func (m *CloudMessage) String() string {
	return fmt.Sprintf("%+v", struct {
		Timestamp time.Time
		Body      string
		DeviceID  uint32
		Length    uint16
		Checksum  string
	}{
		Timestamp: m.Timestamp(),
		Body:      string(m.Body),
		DeviceID:  m.DeviceID,
		Length:    m.Length,
		Checksum:  fmt.Sprintf("%x", m.CloudMessageHeader.Checksum),
	})
}

func (m *CloudMessage) Timestamp() time.Time {
	return time.Unix(int64(m.Epoch), 0)
}

func (m *CloudMessage) Checksum(key []byte) (sum [16]byte, err error) {
	buf := bytes.Buffer{}

	// write header out
	err = binary.Write(&buf, binary.BigEndian, &m.CloudMessageHeader)
	if err != nil {
		return sum, err
	}

	if buf.Len() < 16 {
		return sum, fmt.Errorf("buffer length too short: %d", buf.Len())
	}

	header := buf.Bytes()[0:16]

	h := md5.New()
	h.Write(header)
	h.Write(key)
	h.Write(m.Body)

	for i, v := range h.Sum(nil) {
		sum[i] = v
	}

	return sum, nil
}

func pad(src []byte) []byte {
	padding := aes.BlockSize - len(src)%aes.BlockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padtext...)
}

func unpad(src []byte) ([]byte, error) {
	length := len(src)
	unpadding := int(src[length-1])

	if unpadding > length {
		return nil, errors.New("unpad error. This could happen when incorrect encryption key is used")
	}

	return src[:(length - unpadding)], nil
}

// prepare cloud otc info and send it to cloud
func (r *Rockrobo) CloudOTCInfo(remoteAddr *net.UDPAddr) (*MethodResultOTCInfo, error) {
	// get internal info
	internalInfo, err := r.GetInternalInfo()
	if err != nil {
		return nil, err
	}

	// set token, which is device token in base64 and hex
	internalInfo.Token = hex.EncodeToString([]byte(base64.StdEncoding.EncodeToString(r.dToken)))

	internalInfo.MAC = r.deviceID.MAC
	internalInfo.Model = r.deviceID.Model
	internalInfo.Life = 40707

	// write json params
	paramsOtcInfo, err := json.Marshal(internalInfo)
	if err != nil {
		return nil, err
	}

	id := r.rand.Int()

	method := &Method{
		ID:     id,
		Params: json.RawMessage(paramsOtcInfo),
		Method: MethodOTCInfo,
	}

	messageBody, err := json.Marshal(method)
	if err != nil {
		return nil, err
	}
	messageString := string(messageBody)

	buf := new(bytes.Buffer)
	message := NewCloudMessage()
	message.DeviceID = uint32(r.deviceID.DeviceID)
	message.Body = messageBody

	err = message.Write(buf, []byte(base64.StdEncoding.EncodeToString(r.deviceID.Key)))
	if err != nil {
		return nil, err
	}

	dataCh := make(chan []byte)
	r.cloudRegisterID(id, dataCh)

	_, err = r.cloudConnection.WriteToUDP(buf.Bytes(), remoteAddr)
	if err != nil {
		return nil, err
	}
	r.logger.Debug().Str("remote_addr", remoteAddr.String()).Str("message", string(messageString)).Msgf("sent %s message", MethodOTCInfo)

	select {
	case data := <-dataCh:
		var method Method
		if err := json.Unmarshal(data, &method); err != nil {
			return nil, err
		}

		var result MethodResultOTCInfo
		if err := json.Unmarshal(method.Result, &result); err != nil {
			return nil, err
		}

		r.logger.Debug().Str("remote_addr", remoteAddr.String()).Interface("result", result).Msg("received otc response")

		return &result, nil
	case <-time.After(5 * time.Second):
		break
	}

	return nil, fmt.Errorf("timeout waiting for otc info")
}
