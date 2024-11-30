package node

import (
	"eavesdrop/crypto"
	"eavesdrop/ocr"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"log"
	"testing"
)

func TestRpcBetweenTwoNodes(t *testing.T) {
	s1Opts := &ServerOpts{
		ListenAddr: "127.0.0.1:3000",
		CodecType:  rpc.JsonCodec,
		privateKey: crypto.GeneratePrivateKey(),
	}

	ocr := &ocr.OCR{}
	s1 := NewServer(s1Opts, ocr)

	s2Opts := &ServerOpts{
		ListenAddr: "127.0.0.1:4000",
		CodecType:  rpc.JsonCodec,
		privateKey: crypto.GeneratePrivateKey(),
	}

	s2 := NewServer(s2Opts, ocr)

	if err := s1.Transporter.Connect(s2.Transporter.Addr()); err!=nil {
		t.Fatal(err)
	}

	statusMsg := &rpc.StatusMsg{
		Id: s1.ID(),
	}

	sMsg, _ := statusMsg.Bytes(s1.Codec)

	msg := &rpc.Message{
		Topic:   rpc.Server,
		Headers: rpc.MessageStatus,
		Data:    sMsg,
	}

	msgBytes, _ := msg.Bytes(s1.Codec)
	log.Printf("payload being sent %s \n", msgBytes)

	rpcMsg := rpc.NewRPCMessage(
		utils.NetAddr(s1.ListenAddr),
		msgBytes,
	)

	rpcBytes, _ := rpcMsg.Bytes(s1.Codec)
	log.Printf("rpc msg being sent %s \n", rpcBytes)

	Er := s1.sendMsg(utils.NetAddr(s2.ListenAddr), rpcBytes)
	log.Printf("%v \n", Er)

	select {}
}

