package ocr

import (
	"eavesdrop/crypto"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"log"
	"testing"
	"time"
)

func TestRpcBetweenTwoNodes(t *testing.T) {
	s1Opts := &ServerOpts{
		// ListenAddr: "127.0.0.1:3000",
		CodecType:  rpc.JsonCodec,
		PrivateKey: crypto.GeneratePrivateKey(),
	}

	s1 := NewServer(s1Opts)
	go s1.Start()

	s2Opts := &ServerOpts{
		// ListenAddr: "127.0.0.1:4000",
		CodecType:  rpc.JsonCodec,
		PrivateKey: crypto.GeneratePrivateKey(),
	}

	s2 := NewServer(s2Opts)
	go s2.Start()

	time.Sleep(2 * time.Second)

	select{}

	// Construct a NewEpochMsg
	newEpochMsg := &rpc.NewEpochMesage{
		EpochID: 1,
	}

	rpcMsg, _ := rpc.NewRPCMessageBuilder(
		utils.NetAddr(s1.ListenAddr),
		s1.Codec,
		s1.id.String(),
	).SetHeaders(
		rpc.MessageNewEpoch,
	).SetTopic(
		rpc.Server,
	).SetPayload(newEpochMsg).Bytes()

	log.Printf("public keys of s1 %v \n", s1.ID())
	log.Printf("public keys of s2 %v \n", s2.ID())
	// log.Printf("peerMap of s1 %+v \n", s1.peerMap)
	// Send the message
	if err := s1.SendMsg(s2.ID().String(), rpcMsg); err != nil {
		log.Fatalf("failed to send message: %v", err)
	}

	select {}
}
