package node

import "eavesdrop/rpc"

type ServerRPCProcessor struct {
	// map of rpc message topics to rpc processors
	// a router
	// "0x1" -> Observer's RPCProcessor
	handlers map[rpc.MesageTopic]rpc.RPCProcessor
}

func (s *ServerRPCProcessor) RegisterHandler(topic rpc.MesageTopic, handler rpc.RPCProcessor) {
	s.handlers[topic] = handler
}

func (s *ServerRPCProcessor) ProcessMessage(msg *rpc.DecodedMsg) error {
	handler, ok := s.handlers[msg.Data.Topic]
	if !ok {
		return nil
	}
	return handler.ProcessMessage(msg)
}
