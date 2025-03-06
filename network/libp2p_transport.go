package network

import (
	"context"
	"crypto/sha256"
	"eavesdrop/logger"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	cr "eavesdrop/crypto"

	"github.com/libp2p/go-libp2p"
	c "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"go.uber.org/zap"
)

const (
	protocolID       = "/libp2p/ocr"
	RendezvousString = "p2p-discovery-example"
)

type LibP2pTransport struct {
	host host.Host

	// mutex lock to protect the msg channel
	mu sync.Mutex
	// channel for new incoming peers
	peerCh chan *Peer

	// passed as dependencies from the Server
	msgCh chan *rpc.RPCMessage
	// default codec is json, we can set custom using setCodec
	codec  rpc.Codec
	logger *zap.SugaredLogger
	pk     c.PrivKey
}

// peerCh passed down as dependency from the server, to be used to inform the callers of newly connected
// disconnected peers. Socket info (+ port selection is by protocol itslef) is hidden from teh caller
func NewLibp2pTransport(msgCh chan *rpc.RPCMessage, codec rpc.Codec, pk cr.PrivateKey) *LibP2pTransport {
	priv, _, _ := c.KeyPairFromStdKey(pk.Key())
	return &LibP2pTransport{
		msgCh:  msgCh,
		peerCh: make(chan *Peer, 100),
		logger: logger.Get().Sugar(),
		codec:  codec,
		pk:     priv,
	}
}

func (lt *LibP2pTransport) Start() {
	host, err := libp2p.New(
		libp2p.Identity(lt.pk),
		libp2p.ListenAddrStrings(
			"/ip4/127.0.0.1/tcp/0",
		),
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
	)

	if err != nil {
		panic(err)
	}
	lt.host = host

	// Modify logger to include the server's ID in every log entry
	lt.logger = lt.logger.With("network_id", host.ID().String())

	lt.logger.Info("Host with ID ", host.ID().String(), " listen Addr", host.Addrs())

	// attach handlers for reading / writing when peer connects
	host.SetStreamHandler(protocolID, lt.handleStream)

	// start the p2p discovery service
	lt.DiscoverPeers()

	// go func() {
	// 	for {
	// 		p := <-lt.peerCh
	// 		lt.logger.Infof("New peer discovered: %v", p)
	// 	}
	// }()
}

func (lt *LibP2pTransport) ID() string {
	return lt.host.ID().String()
}

func (lt *LibP2pTransport) Stop() error {
	peersId := lt.host.Network().Peers()
	for _, peer := range peersId {
		err := lt.host.Network().ClosePeer(peer)
		if err != nil {
			lt.logger.Error("Errro disconnecting from peer ", peer.String())
			continue
		}
	}
	return nil
}

func (lt *LibP2pTransport) ConsumeMsgs() <-chan *rpc.RPCMessage {
	return lt.msgCh
}

func (lt *LibP2pTransport) ConsumePeers() <-chan *Peer {
	lt.logger.Debug("ConsumePeers called")
	return lt.peerCh
}

func (lt *LibP2pTransport) Addr() utils.NetAddr {
	return utils.NetAddr(lt.host.Addrs()[0].String())
}

func (lt *LibP2pTransport) handleStream(s network.Stream) {
	peerID := s.Conn().RemotePeer()
	lt.logger.Info("New stream opened with peer:", peerID)

	// Launch a dedicated readLoop for this peer
	go lt.readLoop(s, peerID)
}

// readLoop continuously reads from the peer's stream until it's closed
func (lt *LibP2pTransport) readLoop(s network.Stream, peerID peer.ID) {
	defer func() {
		lt.logger.Info("Closing readLoop for peer:", peerID)
		s.Close()
	}()

	lt.logger.Infof("ReadLoop for %v started", peerID)

	buf := make([]byte, 4096) // Buffer size can be adjusted
	for {
		n, err := s.Read(buf)
		if err != nil {
			if err == io.EOF {
				lt.logger.Infof("Peer %v closed stream", peerID)
				return
			}
			lt.logger.Error("Error reading from stream:", err)
			return
		}

		// Process the received message
		data := buf[:n]

		// decode the data into RPC msg
		// Decode message
		rpcMsg := &rpc.RPCMessage{}
		if err := lt.codec.Decode(data, rpcMsg); err != nil {
			lt.logger.Error("Error decoding message from peer:", peerID, err)
			continue // Skip this message, but keep reading
		}

		lt.mu.Lock()
		// Assign peer's address and push to message channel
		lt.msgCh <- rpcMsg
		lt.mu.Unlock()

		lt.logger.Infof("Received message from %v: %v", peerID, string(data))
	}
}

// Start peer discovery
func (lt *LibP2pTransport) DiscoverPeers() {
	// Using mDNS for local peer discovery
	discoveryNotifee := &discoveryNotifee{
		lt.host,
		sync.Mutex{},
		lt.peerCh,
	}
	service := discovery.NewMdnsService(lt.host, RendezvousString, discoveryNotifee)
	err := service.Start()
	if err != nil {
		panic(err)
	}
}

// this msg is exposed to the caller, for sending msg to a particular peer
func (lt *LibP2pTransport) SendMsg(id string, data []byte) error {
	pID, err := peer.Decode(id)
	if err != nil {
		lt.logger.Error("Invalid peer ID:", err)
		return err
	}

	// Open a stream to the peer
	stream, err := lt.host.NewStream(context.Background(), pID, protocolID)
	if err != nil {
		fmt.Println("Failed to open stream:", err)
		return err
	}

	// Send a message
	lt.logger.Info("Sending message to peer:", peer.ID(id))

	_, err = stream.Write(data)
	if err != nil {
		lt.logger.Error("Error writing to stream:", err)
	}

	return nil
}

// braodcast
func (lt *LibP2pTransport) Broadcast(data []byte) {
	peersId := lt.host.Network().Peers()
	for _, peer := range peersId {
		err := lt.SendMsg(peer.String(), data)
		if err != nil {
			lt.logger.Error("Error broadcasting msg to peer ", peer.String())
			continue
		}
	}
}

// setter for Codec for future uses
func (lt *LibP2pTransport) SetCodec(c rpc.Codec) {
	lt.codec = c
}

// Struct to handle new peer discovery
type discoveryNotifee struct {
	h host.Host
	// mutex lock to protect the msg channel
	mu     sync.Mutex
	peerCh chan *Peer
}

// Called when a new peer is discovered
func (d *discoveryNotifee) HandlePeerFound(p peer.AddrInfo) {
	logger.Get().Sugar().Info("Discovered a new peer:", p.ID, " host id ", d.h.ID())
	// DIRTY solution: adding a random delay of upto 2sec to avoid the TCP simulatenous connect error
	time.Sleep(getPeerDelay(p.ID))

	// Attempt to connect to the peer
	if err := d.h.Connect(context.Background(), p); err != nil {
		fmt.Println("Connection failed:", err)
		return
	}

	// Add the peer to the channel (eventually to be consumed by the server)
	peer := NewPeer(p.ID.String(), p.Addrs[0].String())
	d.peerCh <- peer
}

// Compute a deterministic delay based on peer ID
// a little slower than ranadom delay but slower
func getPeerDelay(peerID peer.ID) time.Duration {
	hash := sha256.Sum256([]byte(peerID.String()))
	delayMs := binary.BigEndian.Uint64(hash[:8]) % 2000 // Mod 2000ms (2 sec)
	return time.Duration(delayMs) * time.Millisecond
}
