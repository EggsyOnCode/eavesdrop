package ocr

import (
	"eavesdrop/network"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"fmt"
	"log"
	"net"
	"reflect"
)

type ServerRPCProcessor struct {
	// map of rpc message topics to rpc processors
	// a router
	// "0x1" -> Observer's RPCProcessor
	handlers      map[rpc.MesageTopic]rpc.RPCProcessor
	peerMap       *map[string]*network.Peer        // points to one in Server
	incomingPeers *map[utils.NetAddr]*network.Peer // points to one in Server
	outgoingPeers *map[utils.NetAddr]*network.Peer // points to one in Server
}

func (s *ServerRPCProcessor) RegisterHandler(topic rpc.MesageTopic, handler rpc.RPCProcessor) {
	s.handlers[topic] = handler
}

func (s *ServerRPCProcessor) ProcessMessage(msg *rpc.DecodedMsg) error {
	switch msg.Topic {
	case rpc.Server:
		// written from the pov of P2
		// ctx: we will only recieve status Msgs from a peer from whom we have recently accepted a conn, and who must be in the peerTempMap , we can fetch them suing conn.RemoteAddr

		log.Printf("msg received from %v is %v \n", msg.FromSock, msg)
		log.Printf("message type: %v", reflect.TypeOf(msg.Data))
		switch msg.Data.(type) {
		case *rpc.StatusMsg:
			statusMsg := msg.Data.(*rpc.StatusMsg)

			// check outgoing peers status.ListenAddr; if not there dial a conn
			// if yes, fetch it from outgoingPeers
			// eg: If p2 connects to p1, p1 will have p2 in outgoingPeers
			// later if p2 receives status msgs  from p1, it shouldn't dial again
			var peer *network.Peer
			if _, ok := (*s.outgoingPeers)[utils.NetAddr(statusMsg.ListenAddr)]; ok {
				peer = (*s.outgoingPeers)[utils.NetAddr(statusMsg.ListenAddr)]
			} else {
				conn, err := net.Dial("tcp", statusMsg.ListenAddr)
				if err != nil {
					return fmt.Errorf("failed to dial remote address: %w", err)
				}
				peer = network.NewPeer(conn)
				delete(*s.outgoingPeers, utils.NetAddr(peer.Addr()))
			}

			// fetch peer who sent this statusMsg, from peerMapAddr
			// node must have received a connection from this peer
			// therefore this node is labelled as incomingPeer
			remotePeer, ok := (*s.incomingPeers)[msg.FromSock]
			if !ok {
				return fmt.Errorf("peer not found during stautus msg reception")
			}

			// set peer's readSock to remotePeer's writeSock
			peer.SetReadSock(remotePeer.WriteSock())

			// delete remotePeer from peerMapAddr
			delete(*s.incomingPeers, msg.FromSock)

			// check if node of ID already exists in peerMap
			_, ok2 := (*s.peerMap)[statusMsg.Id.String()]
			if ok2 {
				return fmt.Errorf("peer %s already connected", statusMsg.Id)
			} else {
				(*s.peerMap)[statusMsg.Id.String()] = peer
				log.Printf("peer with ID %v added to peerMap addr: %v", statusMsg.Id, peer.Addr())
			}
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
			FromSock: rpcMsg.FromSock,
			FromId:   rpcMsg.FromID,
			Topic:    msg.Topic,
			Data:     newMsg,
		}, nil

	case rpc.MessageStatus:

		newMsg := &rpc.StatusMsg{}
		if err := codec.Decode(msg.Data, newMsg); err != nil {
			return nil, err
		}
		return &rpc.DecodedMsg{
			FromSock: rpcMsg.FromSock,
			FromId:   rpcMsg.FromID,
			Topic:    msg.Topic,
			Data:     newMsg,
		}, nil
	default:
		fmt.Println(msg)
		return nil, fmt.Errorf("unknown message type: %v", msg.Headers)
	}
}
