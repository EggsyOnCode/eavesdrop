package rpc

import (
	"eavesdrop/utils"
	"fmt"
)

// Builder for RPCMessage
type RPCMessageBuilder struct {
	fromSock    utils.NetAddr
	fromId      string
	codec       Codec
	headers     MessageType
	topic       MesageTopic
	payloadData interface{} // Generic payload (e.g., StatusMsg, NewEpochMsg, etc.)
	message     []byte
}

// NewRPCMessageBuilder initializes the builder
func NewRPCMessageBuilder(from utils.NetAddr, codec Codec, fromID string) *RPCMessageBuilder {
	return &RPCMessageBuilder{
		fromSock: from,
		fromId:   fromID,
		codec:    codec,
	}
}

// SetHeaders sets the headers (MessageType)
func (b *RPCMessageBuilder) SetHeaders(headers MessageType) *RPCMessageBuilder {
	b.headers = headers
	return b
}

// SetTopic sets the topic (MesageTopic)
func (b *RPCMessageBuilder) SetTopic(topic MesageTopic) *RPCMessageBuilder {
	b.topic = topic
	return b
}

// SetPayload sets the specific payload (e.g., StatusMsg, NewEpochMsg, etc.)
func (b *RPCMessageBuilder) SetPayload(payload interface{}) *RPCMessageBuilder {
	b.payloadData = payload
	return b
}

func (b *RPCMessageBuilder) SetMessage(message []byte) *RPCMessageBuilder {
	b.message = message
	return b
}

// Build constructs the RPCMessage
func (b *RPCMessageBuilder) Build() (*RPCMessage, error) {
	var msgBytes []byte
	if b.message == nil {
		// Encode the specific payload (StatusMsg, NewEpochMsg, etc.)
		payloadBytes, err := b.codec.Encode(b.payloadData)
		if err != nil {
			return nil, fmt.Errorf("failed to encode payload: %w", err)
		}

		// Construct the Message
		message := &Message{
			Headers: b.headers,
			Topic:   b.topic,
			Data:    payloadBytes,
		}

		// Encode the Message
		msgBytes, err = message.Bytes(b.codec)
		if err != nil {
			return nil, fmt.Errorf("failed to encode message: %w", err)
		}
	} else {
		msgBytes = b.message
	}

	// Construct the RPCMessage
	return &RPCMessage{
		FromSock:    b.fromSock,
		FromID:  b.fromId,
		Payload: msgBytes,
	}, nil
}

func (b *RPCMessageBuilder) Bytes() ([]byte, error) {
	rpcMessage, err := b.Build()
	if err != nil {
		return nil, err
	}
	return rpcMessage.Bytes(b.codec)
}

// MessageBuilder provides a builder pattern for creating a Message
type MessageBuilder struct {
	headers MessageType
	topic   MesageTopic
	data    []byte
	codec   Codec
}

// NewMessageBuilder initializes a new MessageBuilder
func NewMessageBuilder(c Codec) *MessageBuilder {
	return &MessageBuilder{
		codec: c,
	}
}

// SetHeaders sets the MessageType (headers) for the Message
func (b *MessageBuilder) SetHeaders(headers MessageType) *MessageBuilder {
	b.headers = headers
	return b
}

// SetTopic sets the MesageTopic for the Message
func (b *MessageBuilder) SetTopic(topic MesageTopic) *MessageBuilder {
	b.topic = topic
	return b
}

// SetData sets the payload (data) for the Message
func (b *MessageBuilder) SetData(data ProtocolMsg) *MessageBuilder {
	d, err := data.Bytes(b.codec)
	if err != nil {
		panic("MessageBuilder: unable to encode msg")
	}
	b.data = d
	return b
}

// Build constructs the Message
func (b *MessageBuilder) Build() (*Message, error) {
	if b.data == nil {
		return nil, fmt.Errorf("data cannot be nil")
	}
	return &Message{
		Headers: b.headers,
		Topic:   b.topic,
		Data:    b.data,
	}, nil
}

// Bytes serializes the Message to bytes using the codec
func (b *MessageBuilder) Bytes() ([]byte, error) {
	message, err := b.Build()
	if err != nil {
		return nil, err
	}
	return message.Bytes(b.codec)
}
