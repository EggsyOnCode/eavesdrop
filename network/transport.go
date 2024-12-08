package network

import (
	"eavesdrop/rpc"
	"eavesdrop/utils"
)

type Transport interface {
	Connect(utils.NetAddr) error
	SendMsg(*Peer, []byte) error
	//the extra parameter is to exclude a peer from broadcasting
	// Broadcast([]byte, []*Peer, utils.NetAddr) error
	ConsumeMsgs() <-chan *rpc.RPCMessage
	ConsumePeers() <-chan *Peer
	Addr() utils.NetAddr
	Start()
	ListenToPeer(*Peer)
	Stop() error
}
