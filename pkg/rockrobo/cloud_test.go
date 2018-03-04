package rockrobo

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"
)

const exampleCloudMsgHex = "21310180000000000012d6875a9b239d9878d8dc74a92a8abd6202af14e4956030246b33f80ee16f9dbe47f9ab220d8e5a7ce560bffbf89feb76a1a449afa001120c514cc1669fc888ce16e5d238a25c7a12a95213ee95365139e8fd174af22715d89ec6014d7778e794fcb3388ec9b1ed6e3c223a6a7072c38dbc00fc9d7f5f15e3e9b0f84f987bc6ed18ec6bc5287120a1ebd34e13dba232e8446d4a6be30aa4892f67e22fb6d8c961492828aec4dad8cba3b6ea1c721c908cb701865d1b5a5d1d32abd6390a87dd5149b4dd9874f664bfd145dd0cc75d52d70fb4c879b1e99519940c2de56c2775411f60925e6aaac4c342223f5f61e76f53dfdffc4626a9125f97f4bdbbb7dd692884a15a6e7a6a7cb0cbe00eb04463fb6b94dc38a92db7d75e56583ad65dcf08d784c6d3f021b9d6bc01a13f863408971c61b9735c573224e5a0c580588bdf5f6feb7b3da622a26668da544d03e02e872be05ad1d2538f6e5426b446db370b9c158644bc85cb50aaad0d0cd775c3a38ce50ad5ff1d9b41"

const exampleKeyString = "YWJjZGVmZ2hpamts"

func Test_DecodeAndReencodeCloudMessage(t *testing.T) {

	exampleData, err := hex.DecodeString(exampleCloudMsgHex)
	if err != nil {
		t.Fatal("error while decoding example: ", err)
	}

	exampleKey := []byte(exampleKeyString)
	if err != nil {
		t.Fatal("error while decoding example: ", err)
	}

	var msg CloudMessage

	reader := bytes.NewReader(exampleData)

	err = msg.Read(reader, exampleKey)
	if err != nil {
		t.Fatal("error while decoding example: ", err)
	}

	if exp, act := msg.DeviceID, uint32(1234567); exp != act {
		t.Errorf("unexpected device ID, exp=%d act=%d", exp, act)
	}

	if exp, act := msg.Header, CloudMessageHeaderMagic; exp != act {
		t.Errorf("unexpected header magic, exp=%v act=%v", exp, act)
	}

	if exp, act := msg.Length, uint16(384); exp != act {
		t.Errorf("unexpected length, exp=%v act=%v", exp, act)
	}

	t.Logf("unencrypted message: %s", msg.String())

	msg.Length = 0
	var nullChecksum [16]byte
	msg.CloudMessageHeader.Checksum = nullChecksum

	// reencrypt
	b := new(bytes.Buffer)
	err = msg.Write(b, exampleKey)
	if err != nil {
		t.Fatal("error while reencoding example: ", err)
	}

	if !reflect.DeepEqual(b.Bytes(), exampleData) {
		t.Fatalf("ciphertext mismatch, before and after encrypting are not the same\n before=%x\n after =%x", exampleData, b.Bytes())
	}
}
