package ocr

import (
	"eavesdrop/crypto"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRpcBetweenTwoNodes(t *testing.T) {
	s1Opts := &ServerOpts{
		ListenAddr: "127.0.0.1:3000",
		CodecType:  rpc.JsonCodec,
		privateKey: crypto.GeneratePrivateKey(),
	}

	s1 := NewServer(s1Opts )

	s2Opts := &ServerOpts{
		ListenAddr: "127.0.0.1:4000",
		CodecType:  rpc.JsonCodec,
		privateKey: crypto.GeneratePrivateKey(),
	}

	s2 := NewServer(s2Opts )

	if err := s1.ConnectToPeerNode(utils.NetAddr(s2.ListenAddr)); err != nil {
		log.Fatalf("failed to connect to peer node: %v", err)
	}

	if err := s2.ConnectToPeerNode(utils.NetAddr(s1.ListenAddr)); err != nil {
		log.Fatalf("failed to connect to peer node: %v", err)
	}

	time.Sleep(2 * time.Second)

	assert.NotNil(t, s1.peerMap[s2.id.String()])
	assert.NotNil(t, s2.peerMap[s1.id.String()])

	// Construct a NewEpochMsg
	newEpochMsg := &rpc.NewEpochMsg{
		New:     "new",
		Current: "current",
	}

	rpcMsg, _ := rpc.NewRPCMessageBuilder(
		utils.NetAddr(s1.ListenAddr),
		s1.Codec,
	).SetHeaders(
		rpc.MessageNewEpoch,
	).SetTopic(
		rpc.Server,
	).SetPayload(newEpochMsg).Bytes()

	log.Printf("public keys of s1 %v \n", s1.ID())
	log.Printf("public keys of s2 %v \n", s2.ID())
	log.Printf("peerMap of s1 %+v \n", s1.peerMap)
	// Send the message
	if err := s1.SendMsg(s2.ID(), rpcMsg); err != nil {
		log.Fatalf("failed to send message: %v", err)
	}

	select {}
}
