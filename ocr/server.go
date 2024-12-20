package ocr

import (
	"eavesdrop/crypto"
	"eavesdrop/network"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/romana/rlog"
)

type ServerOpts struct {
	ListenAddr     string
	CodecType      rpc.CodecType
	BootStrapNodes []string           // NetAddr of bootstrap nodes (fetched from Contract)
	PrivateKey     *crypto.PrivateKey // TODO: what does OCR use for ID? (fetched from the contract)
	id             crypto.PublicKey   // derived from PrivateKey
}

type Server struct {
	*ServerOpts
	Transporter  network.Transport
	RPCProcessor *ServerRPCProcessor
	peerLock     *sync.RWMutex      // protects peerCh
	peerCh       chan *network.Peer // peerCh used by Transporters
	Codec        rpc.Codec
	quitCh       chan struct{} // channel for signals to stop server

	// send via dependency injection to Transport layer
	mu            *sync.RWMutex
	incomingPeers map[utils.NetAddr]*network.Peer // temp peer Maps
	outgoingPeers map[utils.NetAddr]*network.Peer // temp peer Maps
	peerMap       map[string]*network.Peer        // map of peers ; key is the public key of the peer
	peerCountChan chan int                        // channel to keep track of connected peers
}

func (s *Server) ID() crypto.PublicKey {
	return s.id
}

func NewServer(opts *ServerOpts) *Server {
	// TODO: checks for inputs like null checks etc

	peerCh := make(chan *network.Peer, 1024)
	msgCh := make(chan *rpc.RPCMessage, 1024)
	transporter := network.NewTCPTransport(utils.NetAddr(opts.ListenAddr), peerCh, msgCh)
	rpcProcessor := &ServerRPCProcessor{
		handlers: make(map[rpc.MesageTopic]rpc.RPCProcessor),
	}

	s := &Server{
		Transporter:   transporter,
		RPCProcessor:  rpcProcessor,
		peerCh:        peerCh,
		peerLock:      &sync.RWMutex{},
		ServerOpts:    opts,
		quitCh:        make(chan struct{}),
		mu:            &sync.RWMutex{},
		peerMap:       make(map[string]*network.Peer),
		incomingPeers: make(map[utils.NetAddr]*network.Peer),
		outgoingPeers: make(map[utils.NetAddr]*network.Peer),
		peerCountChan: make(chan int),
	}

	// setting server's ID
	s.id = s.PrivateKey.PublicKey()

	// setting up codec
	switch s.CodecType {
	case rpc.JsonCodec:
		s.Codec = rpc.NewJsonCodec()
	default:
		panic(fmt.Errorf("unsupported Codec type given in server opts"))
	}

	transporter.SetCodec(s.Codec)
	s.RPCProcessor.peerMap = &s.peerMap
	s.RPCProcessor.incomingPeers = &s.incomingPeers
	s.RPCProcessor.outgoingPeers = &s.outgoingPeers

	return s
}

func (s *Server) Start() {
	// start transporter
	s.Transporter.Start()
	defer s.Transporter.Stop()

	log.Printf("accepting TCP connections on %v", s.ListenAddr)
	// bootstrap nodes
	go s.bootstrapNodes()

	// channel listerns; rpc and peer from transport layer
free:
	for {
		select {
		case peer := <-s.Transporter.ConsumePeers():
			// peer to be added to PeerMap in Trasnporter
			// peer listenere to be started

			s.mu.Lock()

			if peer.ReadSock() == nil {
				// peer is outgoing
				if _, exists := s.outgoingPeers[utils.NetAddr(peer.WriteSock().RemoteAddr().String())]; exists {
					fmt.Printf("outgoing peer %s already connected", peer.Addr())
					continue
				}

				s.outgoingPeers[utils.NetAddr(peer.Addr())] = peer
				log.Printf("peer %v added to outgoing map of %v ", peer.Addr(), s.ListenAddr)

			} else {
				// can happen if p2 , after receiving status msg dials to p1
				// can happen if p2 connects to p1 before p1 connects to p2

				// peer is incoming
				if _, exists := s.incomingPeers[utils.NetAddr(peer.ReadSock().RemoteAddr().String())]; exists {
					fmt.Printf("peer %s already connected", peer.Addr())
					continue
				}

				s.incomingPeers[utils.NetAddr(peer.Addr())] = peer
				log.Printf("peer %v added to incoming map of %v ", peer.Addr(), s.ListenAddr)

				go s.Transporter.ListenToPeer(peer)
			}

			s.mu.Unlock()

		case rpcMsg := <-s.Transporter.ConsumeMsgs():
			msg, err := s.RPCProcessor.DefaultRPCDecoder(rpcMsg, s.Codec)
			if err != nil {
				rlog.Errorf("error decoding %v \n", err)
				continue
			}

			if err := s.RPCProcessor.ProcessMessage(msg); err != nil {
				rlog.Errorf("error processing rpc msg %v", err)
			}

		case <-s.quitCh:
			break free
		}
	}
	log.Printf("server stopped")
}

// addr is that of the peer; only if its found in the peerMap
func (s *Server) sendHandshakeMsgToPeerNode(addr utils.NetAddr) error {
	// outgoing peers have writeSock which we can write to
	peer, ok := s.outgoingPeers[addr]
	if !ok {
		return fmt.Errorf("peer not found")
	}

	statusMsg := &rpc.StatusMsg{
		Id:         s.ID(),
		ListenAddr: s.ListenAddr,
	}
	// constructing msg
	rpcMsg, err := rpc.NewRPCMessageBuilder(
		utils.NetAddr(s.ListenAddr),
		s.Codec,
		s.ID().String(),
	).SetHeaders(
		rpc.MessageStatus,
	).SetTopic(
		rpc.Server,
	).SetPayload(statusMsg).Bytes()

	if err != nil {
		return err
	}

	// removing peer from outgoingPeers
	delete(s.outgoingPeers, addr)

	return s.Transporter.SendMsg(peer, rpcMsg)
}

// remote Server's ListenAddr
func (s *Server) ConnectToPeerNode(addr utils.NetAddr) error {
	// connect to the peer
	time.Sleep(1 * time.Second)

	if err := s.Transporter.Connect(addr); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	// sending a handshake msg to the peer
	if err := s.sendHandshakeMsgToPeerNode(addr); err != nil {
		return err
	}

	return nil
}

// sending msg using peer's ID
func (s *Server) SendMsg(id string, msg []byte) error {
	// constructing rpcMsg from msg in args
	// msg contains payload, headers, topic i.e the rpc.Message structure
	rpcMsg, err := rpc.NewRPCMessageBuilder(
		utils.NetAddr(s.ListenAddr),
		s.Codec,
		s.id.String(),
	).SetMessage(msg).Build()
	if err != nil {
		return fmt.Errorf("failed to build rpc message: %w", err)
	}

	rpcMsgBytes, err := rpcMsg.Bytes(s.Codec)
	if err != nil {
		return fmt.Errorf("failed to encode rpc message: %w", err)
	}

	// check if peer exists in peerMap
	idHex, e := crypto.StringToPublicKey(id)
	if e != nil {
		return e
	}
	peer, ok := s.peerMap[idHex.String()]
	if !ok {
		return fmt.Errorf("peer not found")
	}

	rlog.Infof("sent msg to peer %v", peer.Addr())

	return s.Transporter.SendMsg(peer, rpcMsgBytes)
}

func (s *Server) BroadcastMsg(msg []byte) error {
	// constructing rpcMsg from msg in args
	// msg contains payload, headers, topic i.e the rpc.Message structure
	rpcMsg, err := rpc.NewRPCMessageBuilder(
		utils.NetAddr(s.ListenAddr),
		s.Codec,
		s.ID().String(),
	).SetMessage(msg).Build()
	if err != nil {
		return fmt.Errorf("failed to build rpc message: %w", err)
	}

	rpcMsgBytes, err := rpcMsg.Bytes(s.Codec)
	if err != nil {
		return fmt.Errorf("failed to encode rpc message: %w", err)
	}

	// send msg to all peers
	for _, peer := range s.peerMap {
		if err := s.Transporter.SendMsg(peer, rpcMsgBytes); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) Stop() {
	close(s.quitCh)
}

func (s *Server) GetCodec() *rpc.Codec {
	return &s.Codec
}

func (s *Server) GetPeerCount() int {
	return len(s.peerMap)
}

func (s *Server) bootstrapNodes() {
	peers, err := NewJsonConfigReader("peers.json").GetPeerInfo()
	if err != nil {
		log.Fatalf("failed to read peers: %v", err)
	}

	connected := 0
	// Iterate over peers and connect
	rlog.Printf("Connecting to %+v peers\n", (peers))
	rlog.Printf("Server ID: %s\n", s.ID().String())
	for _, peer := range peers {
		if peer.PublicKey == s.ID().String() {
			// Skip the server's own entry
			continue
		}

		if err := s.ConnectToPeerNode(utils.NetAddr(peer.ListenAddr)); err != nil {
			rlog.Errorf("failed to connect to peer at %s: %v", peer.ListenAddr, err)
			continue
		}

		connected++

		rlog.Infof("Connected to peer: %s\n", peer.PublicKey)
	}

	// send connected peers count to peerCountChan
	// which is used by the OCR to keep track of connected peers
	s.peerCountChan <- connected
}
