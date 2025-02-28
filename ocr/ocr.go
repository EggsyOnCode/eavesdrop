package ocr

import (
	"eavesdrop/logger"
	"eavesdrop/rpc"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"
)

// context of an OCR node
type OCRCtx struct {
	PeerCount   int // n : no of peers in the network (n >= 3f+1) ; need it to obtain f
	FaultyCount int // f : no of faulty peers in the network (f = (n-1)/3)
	ID          string
}

type OCR struct {
	ctx       *OCRCtx
	Codec     rpc.Codec
	Pacemaker *Pacemaker // always running
	Server    *Server
	// job processor / job scheduler / job registry
	// we need redis caching layer to cache jobs based on their triggers
	// because we need a job scheduler to schedule jobs based on their triggers
	// tracker for phase of hte protocol
	peerCountInit chan struct{}
	quitCh        chan struct{} // channel for signals to stop OCR
	pacemakerCh   chan *rpc.PacemakerMessage
	// ocr State
	state  rpc.OCRState
	logger *zap.SugaredLogger
}

func NewOCR(s *Server) *OCR {
	ocr := &OCR{
		Server:        s,
		ctx:           &OCRCtx{},
		peerCountInit: make(chan struct{}),
		quitCh:        make(chan struct{}),
		pacemakerCh:   make(chan *rpc.PacemakerMessage),
		state:         rpc.PREPARE,
		logger:        logger.Get().Sugar(),
	}
	ocr.Pacemaker = NewPaceMaker(s, ocr.pacemakerCh, s.PrivateKey)
	return ocr
}

func (o *OCR) Start() error {
	//ensure server, pacemarker and reporting engine are not nil
	if o.Server == nil {
		return fmt.Errorf("OCR: missing required components")
	}

	go func() {
		// can be updated in future, so the func should have a permanet listener

		// Wait for the server to report connected peers
		peerCount := <-o.Server.peerCountChan
		o.ctx.PeerCount = peerCount
		faultyC := float32(((peerCount - 1) / 3))
		o.ctx.FaultyCount = int(math.Ceil(float64(faultyC)))

		// including server ID in oct ctx
		o.ctx.ID = o.Server.ID().String()

		close(o.peerCountInit) // Signal that the context is ready
	}()

	// start the server
	go o.Server.Start()

	// reporter and pacemaker start concurrently..
	// altho reporter does nothing unitl the OCR node enters an epoch
	o.Pacemaker.AttachMsgLayer(o.Server) // to enable comms over the p2p network

	// register rpc processor in server for handling incoming rpc msg
	o.Server.RPCProcessor.RegisterHandler(rpc.Pacemaker, o.Pacemaker)
	o.Server.RPCProcessor.RegisterHandler(rpc.Pacemaker, o.Pacemaker.Reporter)

	//TODO: how to register handlers for reporter in Server o.Server.RPCProcessor.RegisterHandler(rpc.Reporter, o.Reporter)

	<-o.peerCountInit // Block until peer count is initialized

	// pass the context to the pacemaker and reporter
	o.Pacemaker.SetOCRContext(o.ctx)

	time.Sleep(5 * time.Second) // wait for the server to find peers

	go o.Pacemaker.Start()

	// start teh readloop
	go o.readLoop()

	return nil
}

// this is for async comms / state updates between OCR and its sub components, pacemaker, reporter and transmitter
func (o *OCR) readLoop() {
free:
	for {
		select {
		case msg := <-o.pacemakerCh:

			// t := reflect.TypeOf(msg.Data)
			// o.logger.Infof("msg type: %v", t)

			// handle pacemaker messages
			switch msg.Data.(type) {
			case rpc.OCRState:
				if state, ok := msg.Data.(rpc.OCRState); ok {
					o.state = state
				} else {
					// Handle type assertion failure
					o.logger.Errorf("OCR: type assertion failed")
				}
			default:
				o.logger.Errorf("msg type: %v ", msg.Data)
				o.logger.Errorf("OCR: unknown message type")
			}

		case <-o.quitCh:
			break free
		}
	}
}

func (o *OCR) Stop() {
	o.Server.Stop()
}
