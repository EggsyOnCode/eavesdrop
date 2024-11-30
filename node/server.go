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
	mu          *sync.RWMutex
	peerMapAddr map[utils.NetAddr]*network.Peer     // temp map to store accepetdConn whose public IDs are not known ; once tehy are found ; items are deleted
	peerMap     map[*crypto.PublicKey]*network.Peer // map of peers ; key is the public key of the peer
	msgCh       chan rpc.RPCMessage                 // channel for messages
}

func (s *Server) ID() crypto.PublicKey {
	return s.id
}

func NewServer(opts *ServerOpts, ocr *ocr.OCR) *Server {
	// TODO: checks for inputs like null checks etc

	peerCh := make(chan *network.Peer)
	msgCh := make(chan *rpc.RPCMessage)
	transporter := network.NewTCPTransport(utils.NetAddr(opts.ListenAddr), peerCh, msgCh)
	rpcProcessor := &ServerRPCProcessor{handlers: make(map[rpc.MesageTopic]rpc.RPCProcessor)}

	// Register OCR-specific handlers
	rpcProcessor.RegisterHandler(rpc.Observer, ocr.Observer)
	rpcProcessor.RegisterHandler(rpc.Reporter, ocr.Reporter)
	rpcProcessor.RegisterHandler(rpc.Transmittor, ocr.Transmitter)

	s := &Server{
		Transporter:  transporter,
		RPCProcessor: rpcProcessor,
		peerCh:       peerCh,
		peerLock:     &sync.RWMutex{},
		OCR:          ocr,
		ServerOpts:   opts,
		quitCh:       make(chan struct{}),
		mu:           &sync.RWMutex{},
		peerMap:      make(map[*crypto.PublicKey]*network.Peer),
		peerMapAddr:  make(map[utils.NetAddr]*network.Peer),
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

			if _, exists := s.peerMapAddr[utils.NetAddr(peer.Addr())]; exists {
				return fmt.Errorf("peer %s already connected", peer.Addr())
			}

			s.peerMapAddr[utils.NetAddr(peer.Addr())] = peer
			go s.Transporter.ListenToPeer(peer,)

			log.Printf("peer added to peerMap addr: %v", peer.Addr())

		case rpcMsg := <-s.Transporter.ConsumeMsgs():
			msg, err := s.RPCProcessor.DefaultRPCDecoder(rpcMsg, s.Codec)
			if err != nil {
				log.Printf("error %v \n", err)
				continue
			}

			switch msg.Data.(type) {
			case rpc.StatusMsg:
				statusMsg := msg.Data.(rpc.StatusMsg)
				peer, ok := s.peerMapAddr[msg.From]
				if !ok {
					log.Printf("peer not found during stautus msg reception")
					continue
				}

				s.peerMap[&statusMsg.Id] = peer
			default:
				if err := s.RPCProcessor.ProcessMessage(msg); err != nil {
					log.Printf("error %v", err)
					continue
				}
			}

		case <-s.quitCh:
			break free
		}
	}
	log.Printf("server stopped")
	return nil
}

// addr is that of the peer; only if its found in the peerMap
func (s *Server) sendMsg(addr utils.NetAddr, msg []byte) error {
	peer, ok := s.peerMapAddr[addr]
	if !ok {
		return fmt.Errorf("peer not found")
	}

	return s.Transporter.SendMsg(peer, msg)
}
