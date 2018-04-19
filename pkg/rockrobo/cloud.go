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

func (r *Rockrobo) runCloudReceiver() {
	defer r.waitGroup.Done()

	for {
		packet := make([]byte, 4096)
		length, raddr, err := r.cloudConnection.ReadFrom(packet)
		if err != nil {
			if x, ok := err.(*net.OpError); ok && x.Op == "read" && x.Err.Error() == "use of closed network connection" {
				break
			}
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
			r.cloudKeepAliveCh <- raddr
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

}

// TODO: get this loop broken down
// TODO: waitgroup should determine if the goroutine has been stopped
func (r *Rockrobo) runCloudKeepAlive() {
	var err error

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

				// wait for a ready device
				<-r.localDeviceReadyCh

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
				case helloAddr := <-r.cloudKeepAliveCh:
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
}

func (r *Rockrobo) cloudRegisterID(id int, dataCh chan []byte) {
	r.cloudSessionsLock.Lock()
	r.cloudSessions[id] = dataCh
	r.cloudSessionsLock.Unlock()
}
