package network

import (
	"eavesdrop/rpc"
	"eavesdrop/utils"
)

type Transport interface {
	// Connect(utils.NetAddr) error
	SendMsg(string, []byte) error // where string is the peerID (for other trasnports other than libp2p, could be the remote addr or pubkey)
	//the extra parameter is to exclude a peer from broadcasting
	ConsumeMsgs() <-chan *rpc.RPCMessage
	ConsumePeers() <-chan *Peer
	// GetPeer(string) string
	Addr() utils.NetAddr
	Start()
	Broadcast([]byte)
	// ListenToPeer(*Peer)
	Stop() error
	DiscoverPeers() // responsible for discovering nodes in the network
	SetCodec(rpc.Codec)
	ID() string
}
