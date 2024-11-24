package rpc

import (
	"bytes"
	"eavesdrop/network"
	"io"
)

type RPCMessage struct {
	From    network.NetAddr
	Payload io.Reader
}

// payload here will be of type Message but in serialized bytes format
func NewRPCMessage(from network.NetAddr, payload []byte) *RPCMessage {
	return &RPCMessage{
		From:    from,
		Payload: bytes.NewReader(payload),
	}
}

// after receiving a msg, it must first be decoded by
// Codec, then passed to RPCProcessor
type DecodedMsg struct {
	From network.NetAddr
	Data Message
}

// All RPCProcessor i.e. Observer, Reporter etc have to implement this
type RPCProcessor interface {
	// Message processed will be decoded msg
	ProcessMessage(*DecodedMsg) error
}
