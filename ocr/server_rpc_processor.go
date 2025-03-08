package ocr

import (
	"eavesdrop/rpc"
	"fmt"
	"log"
	"reflect"

	"go.uber.org/zap"
)

type ServerRPCProcessor struct {
	// map of rpc message topics to rpc processors
	// a router
	// "0x1" -> Observer's RPCProcessor
	handlers          map[rpc.MesageTopic]rpc.RPCProcessor
	logger            *zap.SugaredLogger
	commsChWithServer chan *rpc.InternalPeerServerInfoMsg
}

func (s *ServerRPCProcessor) RegisterHandler(topic rpc.MesageTopic, handler rpc.RPCProcessor) {
	s.handlers[topic] = handler
}

func (s *ServerRPCProcessor) ProcessMessage(msg *rpc.DecodedMsg) error {
	switch msg.Topic {
	case rpc.Server:
		// written from the pov of P2
		// ctx: we will only recieve status Msgs from a peer from whom we have recently accepted a conn, and who must be in the peerTempMap , we can fetch them suing conn.RemoteAddr

		// s.logger.Infof("msg received from %v is %v \n", msg.FromId, msg)
		// s.logger.Infof("message type: %v", reflect.TypeOf(msg.Data))
		switch msg.Data.(type) {
		case *rpc.StatusMsg:
			statusMsg := msg.Data.(*rpc.StatusMsg)
			infoMsg := rpc.InternalPeerServerInfoMsg{
				NetworkId:  statusMsg.NetworkId,
				ListenAddr: statusMsg.ListenAddr,
				ServerId:   statusMsg.Id,
			}

			s.commsChWithServer <- &infoMsg

		default:
			log.Printf("unknown message type: %v", reflect.TypeOf(msg.Data))
		}

		return nil
	default:
		handler, ok := s.handlers[msg.Topic]
		if !ok {
			return nil
		}
		return handler.ProcessMessage(msg)
	}
}

func (s *ServerRPCProcessor) DefaultRPCDecoder(rpcMsg *rpc.RPCMessage, codec rpc.Codec) (*rpc.DecodedMsg, error) {
	// decode the rpcMsg.payload (which shall have the Messgae)
	// switch over the msg.Headers
	// construct appropriate DecodedMsg (which will have topic for routing by s.ProcessMessage) and Data which can
	// have stricter data type like MessageNewEpoch
	// pass it off to s.ProcessMessage

	// 1. decoding into Mesage Type
	msg := &rpc.Message{}
	if err := codec.Decode(rpcMsg.Payload, msg); err != nil {
		return nil, err
	}

	// 2. switch over the MsgHeaders
	switch msg.Headers {
	case rpc.MessageNewEpoch:
		// the msg received is of type NewEpoch
		newMsg := &rpc.NewEpochMesage{}

		if err := codec.Decode(msg.Data, newMsg); err != nil {
			return nil, err
		}
		return &rpc.DecodedMsg{
			FromId: rpcMsg.FromID,
			Topic:  msg.Topic,
			Data:   newMsg,
		}, nil

	case rpc.MessageStatus:
		newMsg := &rpc.StatusMsg{}
		if err := codec.Decode(msg.Data, newMsg); err != nil {
			return nil, err
		}
		return &rpc.DecodedMsg{
			FromId: rpcMsg.FromID,
			Topic:  msg.Topic,
			Data:   newMsg,
		}, nil

	case rpc.MessageObserveReq:
		newMsg := &rpc.ObserveReq{}
		if err := codec.Decode(msg.Data, newMsg); err != nil {
			return nil, err
		}
		return &rpc.DecodedMsg{
			FromId: rpcMsg.FromID,
			Topic:  msg.Topic,
			Data:   newMsg,
		}, nil

	case rpc.MessageObserveRes:
		newMsg := &rpc.ObserveResp{}
		if err := codec.Decode(msg.Data, newMsg); err != nil {
			return nil, err
		}
		return &rpc.DecodedMsg{
			FromId:    rpcMsg.FromID,
			Topic:     msg.Topic,
			Data:      newMsg,
			Signature: rpcMsg.Signature,
		}, nil
	case rpc.MessageChangeLeader:
		newMsg := &rpc.ChangeLeaderMessage{}
		if err := codec.Decode(msg.Data, newMsg); err != nil {
			return nil, err
		}
		return &rpc.DecodedMsg{
			FromId: rpcMsg.FromID,
			Topic:  msg.Topic,
			Data:   newMsg,
		}, nil
	default:
		return nil, fmt.Errorf("unknown message type: %v", msg.Headers)
	}
}
