package rpc

import (
	"bytes"
	c "eavesdrop/crypto"
	"eavesdrop/utils"
	"encoding/gob"
)

type RPCMessage struct {
	FromID    string      `json:"from_id"`
	Payload   []byte      `json:"payload"`
	Signature c.Signature `json:"signature"`
}

// payload here will be of type Message but in serialized bytes format
func NewRPCMessage(from utils.NetAddr, payload []byte, fromID string) *RPCMessage {
	return &RPCMessage{
		FromID:  fromID,
		Payload: payload,
	}
}

func (m *RPCMessage) Bytes(codec Codec) ([]byte, error) {
	return codec.Encode(m)
}

// after receiving a msg, it must first be decoded by
// Codec, then passed to RPCProcessor
type DecodedMsg struct {
	FromSock  utils.NetAddr
	FromId    string
	Topic     MesageTopic
	Data      any // this can be replaced with a more stricter data type like NewEpochMesage (i.e from message.go)
	Signature c.Signature
}

// All RPCProcessor i.e. Observer, Reporter etc have to implement this
type RPCProcessor interface {
	// Message processed will be decoded msg
	ProcessMessage(*DecodedMsg) error
}

type RPCDecodeFunc func(RPCMessage, Codec) (*DecodedMsg, error)

type InternalPeerServerInfoMsg struct {
	NetworkId  string
	ListenAddr string
	ServerId   string
}

func (m *InternalPeerServerInfoMsg) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(m); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
