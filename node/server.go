package node

import (
	"eavesdrop/crypto"
	"eavesdrop/network"
	"eavesdrop/ocr"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"fmt"
	"log"
	"sync"
	"time"
)

type ServerOpts struct {
	ListenAddr     string
	CodecType      rpc.CodecType
	BootStrapNodes []string           // NetAddr of bootstrap nodes (fetched from Contract)
	Logger         log.Logger         // interface for all loggers
	privateKey     *crypto.PrivateKey // TODO: what does OCR use for ID? (fetched from the contract)
	id             crypto.PublicKey   // derived from PrivateKey
}

type Server struct {
	*ServerOpts
	Transporter  network.Transport
	RPCProcessor *ServerRPCProcessor
	OCR          *ocr.OCR
	peerLock     *sync.RWMutex      // protects peerCh
	peerCh       chan *network.Peer // peerCh used by Transporters
	Codec        rpc.Codec
	quitCh       chan struct{} // channel for signals to stop server

	// send via dependency injection to Transport layer
	mu            *sync.RWMutex
	incomingPeers map[utils.NetAddr]*network.Peer     // temp peer Maps
	outgoingPeers map[utils.NetAddr]*network.Peer     // temp peer Maps
	peerMap       map[string]*network.Peer // map of peers ; key is the public key of the peer
}

func (s *Server) ID() crypto.PublicKey {
	return s.id
}

func NewServer(opts *ServerOpts, ocr *ocr.OCR) *Server {
	// TODO: checks for inputs like null checks etc

	peerCh := make(chan *network.Peer, 1024)
	msgCh := make(chan *rpc.RPCMessage, 1024)
	transporter := network.NewTCPTransport(utils.NetAddr(opts.ListenAddr), peerCh, msgCh)
	rpcProcessor := &ServerRPCProcessor{
		handlers: make(map[rpc.MesageTopic]rpc.RPCProcessor),
	}

	// Register OCR-specific handlers
	rpcProcessor.RegisterHandler(rpc.Observer, ocr.Observer)
	rpcProcessor.RegisterHandler(rpc.Reporter, ocr.Reporter)
	rpcProcessor.RegisterHandler(rpc.Transmittor, ocr.Transmitter)

	s := &Server{
		Transporter:   transporter,
		RPCProcessor:  rpcProcessor,
		peerCh:        peerCh,
		peerLock:      &sync.RWMutex{},
		OCR:           ocr,
		ServerOpts:    opts,
		quitCh:        make(chan struct{}),
		mu:            &sync.RWMutex{},
		peerMap:       make(map[string]*network.Peer),
		incomingPeers: make(map[utils.NetAddr]*network.Peer),
		outgoingPeers: make(map[utils.NetAddr]*network.Peer),
	}

	// setting server's ID
	s.id = s.privateKey.PublicKey()

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

	// TODO: fire off listeners in the server.Start() goroutine
	go s.Start()

	return s
}

func (s *Server) Start() error {
	// start transporter
	s.Transporter.Start()

	log.Printf("accepting TCP connections on %v", s.ListenAddr)
	// bootstrap nodes

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
				log.Printf("error %v \n", err)
				continue
			}

			if err := s.RPCProcessor.ProcessMessage(msg); err != nil {
				log.Printf("error %v", err)
			}

		case <-s.quitCh:
			break free
		}
	}
	log.Printf("server stopped")
	return nil
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
	).SetHeaders(
		rpc.MessageStatus,
	).SetTopic(
		rpc.Server,
	).SetPayload(statusMsg).Bytes()

	if err != nil {
		return err
	}

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

	fmt.Printf("connected to peer %s", addr)

	return nil
}

// sending msg using peer's ID
func (s *Server) sendMsg(id crypto.PublicKey, msg []byte) error {
	// check if peer exists in peerMap
	log.Printf("Complete peerMap of s1: %+v\n", s.peerMap)
	log.Printf("id is : %+v\n", id)
	peer, ok := s.peerMap[id.String()]
	if !ok {
		return fmt.Errorf("peer not found")
	}

	return s.Transporter.SendMsg(peer, msg)
}
