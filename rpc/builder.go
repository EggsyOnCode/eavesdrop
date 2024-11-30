package rpc

import (
	"eavesdrop/utils"
	"fmt"
)

// Builder for RPCMessage
type RPCMessageBuilder struct {
	from        utils.NetAddr
	codec       Codec
	headers     MessageType
	topic       MesageTopic
	payloadData interface{} // Generic payload (e.g., StatusMsg, NewEpochMsg, etc.)
}

// NewRPCMessageBuilder initializes the builder
func NewRPCMessageBuilder(from utils.NetAddr, codec Codec) *RPCMessageBuilder {
	return &RPCMessageBuilder{
		from:  from,
		codec: codec,
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

// Build constructs the RPCMessage
func (b *RPCMessageBuilder) Build() (*RPCMessage, error) {
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
	messageBytes, err := message.Bytes(b.codec)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// Construct the RPCMessage
	return &RPCMessage{
		From:    b.from,
		Payload: messageBytes,
	}, nil
}

func (b *RPCMessageBuilder) Bytes() ([]byte, error) {
	rpcMessage, err := b.Build()
	if err != nil {
		return nil, err
	}
	return rpcMessage.Bytes(b.codec)
}