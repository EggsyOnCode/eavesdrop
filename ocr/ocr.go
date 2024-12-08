package ocr

import (
	"eavesdrop/rpc"
	"fmt"
)

type OCR struct {
	ObserverReg    *ObserverRegistry
	TransmitterReg *TransmitterRegistry
	Codec          rpc.Codec
	Reporter       *ReportingEngine
	Pacemaker      *Pacemaker
	Server         *Server
}

func NewOCR(s *Server, r *ReportingEngine, p *Pacemaker) *OCR {
	return &OCR{
		Reporter:  r,
		Pacemaker: p,
		Server:    s,
	}
}

func (o *OCR) Start() error {
	//ensure server, pacemarker and reporting engine are not nil
	if o.Server == nil || o.Pacemaker == nil || o.Reporter == nil {
		return fmt.Errorf("OCR: missing required components")
	}

	// start the server
	go o.Server.Start()
	// reporter and pacemaker start concurrently..
	// altho reporter does nothing unitl the OCR node enters an epoch
	o.Pacemaker.AttachMsgLayer(o.Server) // to enable comms over the p2p network
	o.Reporter.AttachMsgLayer(o.Server)

	// register rpc processor in server for handling incoming rpc msg
	o.Server.RPCProcessor.RegisterHandler(rpc.Pacemaker, o.Pacemaker)
	o.Server.RPCProcessor.RegisterHandler(rpc.Reporter, o.Reporter)

	go o.Reporter.Start()
	go o.Pacemaker.Start()

	return nil
}

func (o *OCR) Stop() {
	o.Server.Stop()
}
