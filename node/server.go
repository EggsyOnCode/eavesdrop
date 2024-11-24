package node

import (
	"eavesdrop/network"
	"eavesdrop/ocr"
	"eavesdrop/rpc"
	"sync"
)

type Server struct {
	Transporter  network.Transport
	RPCProcessor *ServerRPCProcessor
	OCR          *ocr.OCR
	// lock
	peerLock *sync.RWMutex         // protects peerCh
	peerCh   chan *network.TcpPeer // peerCh used by Transporters
}

func NewServer(addr string, ocr *ocr.OCR) *Server {
	peerCh := make(chan *network.TcpPeer)
	transporter := network.NewTCPTransport(network.NetAddr(addr), peerCh)
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
	}

	// TODO: fire off listeners in the server.Start() goroutine

	return s
}
