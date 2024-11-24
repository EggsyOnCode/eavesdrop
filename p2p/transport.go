package p2p

type NetAddr string

type Transport interface {
	Connect(Transport) error
	SendMsg(NetAddr, []byte) error
	//the extra parameter is to exclude a peer from broadcasting
	Broadcast([]byte, NetAddr) error
	Consume() <-chan []byte
	Addr() NetAddr
	IsPeer(Transport) bool
	AddPeer(tr Transport) error
	DeletePeer(NetAddr) 
	Start() 
}
