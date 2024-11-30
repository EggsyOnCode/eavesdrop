package rpc

import (
	"eavesdrop/utils"
)

type RPCMessage struct {
	From    utils.NetAddr `json:"from"`
	Payload []byte        `json:"payload"`
}

// payload here will be of type Message but in serialized bytes format
func NewRPCMessage(from utils.NetAddr, payload []byte, ) *RPCMessage {
	return &RPCMessage{
		From:    from,
		Payload: payload,
	}
}

func (m *RPCMessage) Bytes(codec Codec) ([]byte, error) {
	return codec.Encode(m)
}

// after receiving a msg, it must first be decoded by
// Codec, then passed to RPCProcessor
type DecodedMsg struct {
	From  utils.NetAddr
	Topic MesageTopic
	Data  any // this can be replaced with a more stricter data type like NewEpochMesage (i.e from message.go)
}

// All RPCProcessor i.e. Observer, Reporter etc have to implement this
type RPCProcessor interface {
	// Message processed will be decoded msg
	ProcessMessage(*DecodedMsg) error
}

type RPCDecodeFunc func(RPCMessage, Codec) (*DecodedMsg, error)
