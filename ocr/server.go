package ocr

import (
	"eavesdrop/crypto"
	"eavesdrop/logger"
	"eavesdrop/network"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"fmt"
	"log"
	"sync"

	g "github.com/zyedidia/generic"
	"github.com/zyedidia/generic/avl"
	"go.uber.org/zap"
)

// the protocol level rep of network Peer; this DS can be extended to include other info later on
// network.Peer.ID should be === Server.ID
type ProtcolPeer struct {
	*network.Peer
}

type ServerOpts struct {
	ListenAddr     string
	CodecType      rpc.CodecType
	BootStrapNodes []string           // NetAddr of bootstrap nodes (fetched from Contract)
	PrivateKey     *crypto.PrivateKey // TODO: what does OCR use for ID? (fetched from the contract)
	id             crypto.PublicKey   // derived from PrivateKey
}

type Server struct {
	*ServerOpts
	Transporter   network.Transport
	RPCProcessor  *ServerRPCProcessor
	peerLock      *sync.RWMutex      // protects peerCh
	peerCh        chan *network.Peer // peerCh used by Transporters
	Codec         rpc.Codec
	quitCh        chan struct{} // channel for signals to stop server
	peers         *avl.Tree[string, *ProtcolPeer]
	peerCountChan chan int
	logger        *zap.SugaredLogger
}

func (s *Server) ID() crypto.PublicKey {
	return s.id
}

func NewServer(opts *ServerOpts) *Server {
	// TODO: checks for inputs like null checks etc

	peerCh := make(chan *network.Peer, 1024)
	msgCh := make(chan *rpc.RPCMessage, 1024)
	transporter := network.NewLibp2pTransport(peerCh, msgCh, nil, opts.PrivateKey)
	rpcProcessor := &ServerRPCProcessor{
		handlers: make(map[rpc.MesageTopic]rpc.RPCProcessor),
		logger:   logger.Get().Sugar(),
	}

	s := &Server{
		Transporter:   transporter,
		RPCProcessor:  rpcProcessor,
		peerCh:        peerCh,
		peerLock:      &sync.RWMutex{},
		ServerOpts:    opts,
		quitCh:        make(chan struct{}),
		peers:         avl.New[string, *ProtcolPeer](g.Greater[string]),
		logger:        logger.Get().Sugar(),
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
	s.RPCProcessor.peerMap = s.peers

	return s
}

func (s *Server) Start() {
	// start transporter
	s.Transporter.Start()
	defer s.Transporter.Stop()

	// log.Printf("accepting TCP connections on %v", s.ListenAddr)
	// // bootstrap nodes
	// go s.bootstrapNodes()

	// channel listerns; rpc and peer from transport layer
free:
	for {
		select {
		case peer := <-s.Transporter.ConsumePeers():
			// add peer to peerMap
			s.peerLock.Lock()
			pPeer := &ProtcolPeer{peer}
			s.peers.Put(peer.ID, pPeer)
			// s.peerCountChan <- s.peers.Size() (TODO: some issue here)
			log.Print("got another peer")

			// send StatusMsg to peer
			s.sendHandshakeMsgToPeerNode(peer.ID)

			s.peerLock.Unlock()

		case rpcMsg := <-s.Transporter.ConsumeMsgs():
			log.Printf("hello...")
			msg, err := s.RPCProcessor.DefaultRPCDecoder(rpcMsg, s.Codec)
			if err != nil {
				s.logger.Errorf("error decoding %v \n", err)
				continue
			}

			if err := s.RPCProcessor.ProcessMessage(msg); err != nil {
				s.logger.Errorf("error processing rpc msg %v", err)
			}

		case <-s.quitCh:
			break free
		}
	}
	log.Printf("server stopped")
}

// addr is that of the peer; only if its found in the peerMap
func (s *Server) sendHandshakeMsgToPeerNode(id string) error {
	statusMsg := &rpc.StatusMsg{
		Id:         s.ID(),
		ListenAddr: string(s.Transporter.Addr()),
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

	log.Printf("sending msg to peer")

	return s.Transporter.SendMsg(id, rpcMsg)
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

	return s.Transporter.SendMsg(id, rpcMsgBytes)
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

	s.Transporter.Broadcast(rpcMsgBytes)

	return nil
}

func (s *Server) Stop() {
	close(s.quitCh)
}

func (s *Server) GetCodec() *rpc.Codec {
	return &s.Codec
}

func (s *Server) GetPeerCount() int {
	return s.peers.Size()
}

// func (s *Server) bootstrapNodes() {
// 	peers, err := NewJsonConfigReader("peers.json").GetPeerInfo()
// 	if err != nil {
// 		log.Fatalf("failed to read peers: %v", err)
// 	}

// 	connected := 0
// 	// Iterate over peers and connect
// 	rlog.Printf("Connecting to %+v peers\n", (peers))
// 	rlog.Printf("Server ID: %s\n", s.ID().String())
// 	for _, peer := range peers {
// 		if peer.PublicKey == s.ID().String() {
// 			// Skip the server's own entry
// 			continue
// 		}

// 		if err := s.ConnectToPeerNode(utils.NetAddr(peer.ListenAddr)); err != nil {
// 			rlog.Errorf("failed to connect to peer at %s: %v", peer.ListenAddr, err)
// 			continue
// 		}

// 		connected++

// 		rlog.Infof("Connected to peer: %s\n", peer.PublicKey)
// 	}

// 	// send connected peers count to peerCountChan
// 	// which is used by the OCR to keep track of connected peers
// 	s.peerCountChan <- connected
// }
