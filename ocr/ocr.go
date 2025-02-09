package ocr

import (
	"eavesdrop/rpc"
	"fmt"
	"math"
)

// context of an OCR node
type OCRCtx struct {
	PeerCount   int // n : no of peers in the network (n >= 3f+1) ; need it to obtain f
	FaultyCount int // f : no of faulty peers in the network (f = (n-1)/3)
}

type OCR struct {
	ctx            *OCRCtx
	ObserverReg    *ObserverRegistry
	TransmitterReg *TransmitterRegistry
	Codec          rpc.Codec
	Reporter       *ReportingEngine
	Pacemaker      *Pacemaker
	Server         *Server
	peerCountInit  chan struct{}
}

func NewOCR(s *Server, r *ReportingEngine, p *Pacemaker) *OCR {
	return &OCR{
		Reporter:      r,
		Pacemaker:     p,
		Server:        s,
		ctx:           &OCRCtx{},
		peerCountInit: make(chan struct{}),
	}
}

func (o *OCR) Start() error {
	//ensure server, pacemarker and reporting engine are not nil
	if o.Server == nil || o.Pacemaker == nil || o.Reporter == nil {
		return fmt.Errorf("OCR: missing required components")
	}

	go func() {
		// Wait for the server to report connected peers
		peerCount := <-o.Server.peerCountChan
		o.ctx.PeerCount = peerCount
		faultyC := float32(((peerCount - 1) / 3))
		o.ctx.FaultyCount = int(math.Ceil(float64(faultyC)))

		close(o.peerCountInit) // Signal that the context is ready
	}()

	// start the server
	go o.Server.Start()
	// reporter and pacemaker start concurrently..
	// altho reporter does nothing unitl the OCR node enters an epoch
	o.Pacemaker.AttachMsgLayer(o.Server) // to enable comms over the p2p network
	o.Reporter.AttachMsgLayer(o.Server)

	// register rpc processor in server for handling incoming rpc msg
	o.Server.RPCProcessor.RegisterHandler(rpc.Pacemaker, o.Pacemaker)
	o.Server.RPCProcessor.RegisterHandler(rpc.Reporter, o.Reporter)

	<-o.peerCountInit // Block until peer count is initialized

	// pass the context to the pacemaker and reporter
	o.Pacemaker.SetOCRContext(o.ctx)

	go o.Reporter.Start()
	go o.Pacemaker.Start()

	return nil
}

func (o *OCR) Stop() {
	o.Server.Stop()
}
