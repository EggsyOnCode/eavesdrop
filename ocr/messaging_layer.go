package ocr

// MessagingLayer is an interface that defines the methods that a messaging layer should implement
// for now, Server has only one messaging layer, but in future, it can have multiple messaging layers
// for insance, an event based arch involving Kafka or RabbitMQ
type MessagingLayer interface {
	SendMsg(id string, msg []byte) error
	BroadcastMsg(msg []byte) error
}
