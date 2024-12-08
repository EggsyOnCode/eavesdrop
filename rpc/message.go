package rpc

import (
	"eavesdrop/crypto"
	"fmt"
)

type MessageType byte
type MesageTopic byte

const (
	MessageNewEpoch MessageType = 0x1
	MessageStatus   MessageType = 0x2

	Pacemaker MesageTopic = iota
	Reporter  MesageTopic = 0x1

	Server MesageTopic = 0x4
)

type Message struct {
	Headers MessageType // this is for Routing / switching on MsgType
	Topic   MesageTopic
	Data    []byte
}

func NewMessage(headers MessageType, data []byte) *Message {
	return &Message{
		Headers: headers,
		Data:    data,
	}
}

func (m *Message) Bytes(c Codec) ([]byte, error) {
	return c.Encode(m)
}

func (m *Message) String() string {
	return fmt.Sprintf("topic %v, headers %v , data %s ", m.Topic, m.Headers, m.Data)
}

// interface for specific messages
type ProtocolMsg interface {
	Bytes(Codec) ([]byte, error)
}

type StatusMsg struct {
	Id         crypto.PublicKey
	ListenAddr string
}

func (status *StatusMsg) Bytes(c Codec) ([]byte, error) {
	return c.Encode(status)
}
