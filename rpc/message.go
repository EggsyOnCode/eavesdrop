package rpc

type MessageType byte
type MesageTopic byte

const (
	MessageTypeFirst MessageType = 0x1

	Observer    MesageTopic = 0x1
	Reporter    MesageTopic = 0x2
	Transmittor MesageTopic = 0x3
)

type Message struct {
	Headers MessageType
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
