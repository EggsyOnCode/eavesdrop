package ocr

import (
	"eavesdrop/crypto"
	"eavesdrop/logger"
	"eavesdrop/network"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"fmt"
	"sync"
	"time"

	g "github.com/zyedidia/generic"
	"github.com/zyedidia/generic/avl"
	"go.uber.org/zap"
)

// the protocol level rep of network Peer; this DS can be extended to include other info later on
type ProtcolPeer struct {
	ServerID string // server ID (From appToPeerId)
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
	peerLock      *sync.RWMutex // protects peerCh
	Codec         rpc.Codec
	quitCh        chan struct{} // channel for signals to stop server
	peers         *avl.Tree[string, *ProtcolPeer]
	appIdToPeerId map[string]string
	peerCountChan chan int
	logger        *zap.SugaredLogger
}

func (s *Server) ID() crypto.PublicKey {
	return s.id
}

func NewServer(opts *ServerOpts) *Server {
	// TODO: checks for inputs like null checks etc

	msgCh := make(chan *rpc.RPCMessage, 1024)
	transporter := network.NewLibp2pTransport(msgCh, &rpc.JSONCodec{}, *opts.PrivateKey)
	rpcProcessor := &ServerRPCProcessor{
		handlers:          make(map[rpc.MesageTopic]rpc.RPCProcessor),
		logger:            logger.Get().Sugar(),
		commsChWithServer: make(chan *rpc.InternalPeerServerInfoMsg, 1000),
	}

	s := &Server{
		Transporter:   transporter,
		RPCProcessor:  rpcProcessor,
		peerLock:      &sync.RWMutex{},
		ServerOpts:    opts,
		quitCh:        make(chan struct{}),
		appIdToPeerId: make(map[string]string),
		peers:         avl.New[string, *ProtcolPeer](g.Greater[string]),
		peerCountChan: make(chan int, 1000),
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

	s.logger = logger.Get().Sugar().With("server_id", s.id.String())

	transporter.SetCodec(s.Codec)

	return s
}

func (s *Server) Start() {
	// start transporter
	go s.Transporter.Start()
	defer s.Transporter.Stop()

	// added timer because the peerConsumer gorotuine was launching too
	// early and the (buffered) peerCh inside Transporter was not initialized yet
	// causing a deadlock
	time.Sleep(2 * time.Second)

	go func() {
		for peer := range s.Transporter.ConsumePeers() {
			// s.logger.Infof("manual peer connected: %v", peer)
			s.handleNewPeer(peer)
		}
	}()

	go func() {
		for infoMsg := range s.RPCProcessor.commsChWithServer {
			// add peer to peerMap
			s.peerLock.Lock()

			// s.peers.Each(func(k string, v *ProtcolPeer) {
			// 	s.logger.Infof("peerMap: %v -> %v", k, v)
			// })

			peer, ok := s.peers.Get(infoMsg.NetworkId)
			if !ok {
				s.logger.Error("peer not found in peerMap")
				continue
			}

			// add to appIdToPeerId map
			s.appIdToPeerId[infoMsg.ServerId] = infoMsg.NetworkId
			peer.ServerID = infoMsg.ServerId

			s.peerLock.Unlock()
		}
	}()

	// channel listerns; rpc and peer from transport layer
free:
	for {
		select {
		// case infoMsg := <-s.RPCProcessor.commsChWithServer:

		// case peer := <-s.Transporter.ConsumePeers():
		// 	// add peer to peerMap
		// 	s.logger.Debugf("peer connected: %v", peer)
		// 	s.peerLock.Lock()
		// 	pPeer := &ProtcolPeer{peer}
		// 	s.peers.Put(peer.ID, pPeer)
		// 	s.peerCountChan <- s.peers.Size()

		// 	// send StatusMsg to peer
		// 	s.sendHandshakeMsgToPeerNode(peer.ID)

		// 	s.peerLock.Unlock()

		case rpcMsg := <-s.Transporter.ConsumeMsgs():
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
	s.logger.Info("server stopped")
}

// addr is that of the peer; only if its found in the peerMap
// here id is peer's network_id not server_id since we don't know the server_id yet
func (s *Server) sendHandshakeMsgToPeerNode(id string) error {
	statusMsg := &rpc.StatusMsg{
		Id:         s.ID().String(),
		NetworkId:  s.Transporter.ID(),
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

	s.logger.Debug("sending hadnshake with payload: ", rpcMsg)

	if err != nil {
		return err
	}

	return s.Transporter.SendMsg(id, rpcMsg)
}

// sending msg using peer's server Id not network id
// rpcMsg is the encoded rpc.Message in bytes
func (s *Server) SendMsg(id string, rpcMsg []byte) error {
	// get corresponding peerId to send msg
	pID := s.appIdToPeerId[id]
	if pID == "" {
		return fmt.Errorf("peer not found")
	}

	return s.Transporter.SendMsg(pID, rpcMsg)
}

// rpcMsg is the encoded rpc.Message in bytes
func (s *Server) BroadcastMsg(rpcMsg []byte) error {
	s.Transporter.Broadcast(rpcMsg)

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

// id is server_id not network ID
func (s *Server) IsPeer(id string) bool {
	s.peerLock.RLock()
	defer s.peerLock.RUnlock()

	_, ok := s.appIdToPeerId[id]
	return ok
}

func (s *Server) handleNewPeer(peer *network.Peer) {
	// add peer to peerMap
	s.peerLock.Lock()
	pPeer := &ProtcolPeer{
		ServerID: "",
		Peer:     peer,
	}
	s.peers.Put(peer.ID, pPeer)
	s.peerCountChan <- s.peers.Size()

	// send StatusMsg to peer
	s.sendHandshakeMsgToPeerNode(peer.ID)

	s.peerLock.Unlock()
}
