package rockrobo

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"
)

const exampleCloudMsgHex = "21310180000000000012d6875a9b239d9878d8dc74a92a8abd6202af14e4956030246b33f80ee16f9dbe47f9ab220d8e5a7ce560bffbf89feb76a1a449afa001120c514cc1669fc888ce16e5d238a25c7a12a95213ee95365139e8fd174af22715d89ec6014d7778e794fcb3388ec9b1ed6e3c223a6a7072c38dbc00fc9d7f5f15e3e9b0f84f987bc6ed18ec6bc5287120a1ebd34e13dba232e8446d4a6be30aa4892f67e22fb6d8c961492828aec4dad8cba3b6ea1c721c908cb701865d1b5a5d1d32abd6390a87dd5149b4dd9874f664bfd145dd0cc75d52d70fb4c879b1e99519940c2de56c2775411f60925e6aaac4c342223f5f61e76f53dfdffc4626a9125f97f4bdbbb7dd692884a15a6e7a6a7cb0cbe00eb04463fb6b94dc38a92db7d75e56583ad65dcf08d784c6d3f021b9d6bc01a13f863408971c61b9735c573224e5a0c580588bdf5f6feb7b3da622a26668da544d03e02e872be05ad1d2538f6e5426b446db370b9c158644bc85cb50aaad0d0cd775c3a38ce50ad5ff1d9b41"

const exampleKeyString = "YWJjZGVmZ2hpamts"

//213101900000000004ea3a325a9b239ddeba49ca60e65153a40a1801012e6f11d532a3e64f109908954dbd714b40c2b2f0ec6d6fe7d0ee61fb9495be066c0e02c1c2f633ed9c51f975a1511eedc1b9f6589418a9091e1ee7d229b1d17e16b1377564f0caaafa57b1704438025d4103fce7604588126f95c96272610c16de6a1deed7948da9b8fe17728b6b5b2f0c54300e4e4802a51cbaff736a65b77f1af942e11e5d5d064d4b5cb7aee67f579e6557c355815caf4c351a5addf788c2b7907aa4db4808840f6a7f82f03a3d8a22764ff043c1179aa2ad802b2be97865bd3197626222d065c6774f3343b6ca89e7e642f1cab6298ec5ca523e977d64dc079e62dd317a1ebff5fa9a4963ff2a383f35875c189432aecc4e79ff11b53c685911e0cfb4df6560d18083b0db235b3ef2f59da0aca77810823e5c88a45193a83ccb91ee587dfbb69c1f9e84fead0637463e4eea84bac8fd6265b36290028ba5488411155fd2553a09581c0ddda5fdc32b07409f98d9c268b4009c7c93938028ac6460978c993e703cfa6c71dddc4f2c1fd07d"

func Test_DecodeCloudMessage(t *testing.T) {

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

	t.Logf("device id: %d %d", msg.DeviceID, 123)
	t.Logf("body: %s", msg.Body)

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
