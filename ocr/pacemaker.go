package ocr

import (
	"eavesdrop/crypto"
	"eavesdrop/logger"
	"eavesdrop/ocr/jobs"
	"eavesdrop/rpc"
	"fmt"
	"io"
	"sync"
	"time"

	fifo "github.com/foize/go.fifo"
	"github.com/zyedidia/generic/avl"
	"go.uber.org/zap"
)

type SendingSchme int

const (
	BROADCAST SendingSchme = 0
	LEADER    SendingSchme = 1
	PEER      SendingSchme = 2

	ResendTimes    time.Duration = time.Second * 3
	ProgressTime   time.Duration = time.Second * 20
	initTickerTime time.Duration = 500 * time.Millisecond // Start with 500ms
	maxBackoff     time.Duration = 10 * time.Second       // Max 10s backoff
)

var (
	secretKey []byte = []byte("secretvalue")
)

type EpochStats struct {
	f  uint64 // faluty nodes
	n  uint64 // total peers
	ne uint64 // highest epoch number received
	e  uint64 // current epoch number

	// cache
	n_e0  uint64 // no of times e0 > e has been recorded
	new_e uint64 // e^ = f + 1 - ne
}

type Pacemaker struct {
	leader        *crypto.PublicKey // current leader
	msgService    MessagingLayer
	TimerResend   *Timer
	TimerProgress *Timer
	quitCh        chan struct{}
	ocrCtx        *OCRCtx
	currEpochStat *EpochStats

	Reporter *ReportingEngine // ephemeral existence (can be destroyed / restarted)
	server   *Server
	ocrCh    chan *rpc.PacemakerMessage // used to comm with OCR
	logger   *zap.SugaredLogger

	signer *crypto.PrivateKey // we need it to sign msgs, also derive ID

	recEventsChan chan jobs.JobEventResponse
	recEvents     fifo.Queue
	jobRegistry   map[string]jobs.Job // used to query the job info when an event is received
}

func NewPaceMaker(s *Server, ocrCh chan *rpc.PacemakerMessage, signer *crypto.PrivateKey) *Pacemaker {
	return &Pacemaker{
		leader: nil,
		// TODO: don't init the timer right here since it would start
		// ticking immediately, do it when it needs to
		// TimerResend:   NewTimer(time.Duration(ResendTimes)),
		TimerResend: &Timer{},
		// TimerProgress: NewTimer(time.Duration(ProgressTime)),
		TimerProgress: &Timer{},
		quitCh:        make(chan struct{}),
		currEpochStat: &EpochStats{},
		server:        s,
		ocrCh:         ocrCh,
		logger:        logger.Get().Sugar(),
		signer:        signer,
		recEventsChan: make(chan jobs.JobEventResponse),
		recEvents:     *fifo.NewQueue(),
		jobRegistry:   map[string]jobs.Job{},
		Reporter: &ReportingEngine{
			mu:    sync.Mutex{},
			ready: false,
		},
	}
}

func (p *Pacemaker) SetOCRContext(ctx *OCRCtx) {
	p.ocrCtx = ctx

	// when init current epoch stats are same as OCR context
	p.currEpochStat.f = uint64(ctx.FaultyCount)
	p.currEpochStat.n = uint64(ctx.PeerCount)
}

// if sending Scheme is BROADCAST, send message to all oracles
// if sending Scheme is LEADER, send message to leader (id of hte leader is passed)
// if sending Scheme is PEER, send message to peer (id of the peer is passed)
func (p *Pacemaker) SendMsg(s SendingSchme, msg []byte, id string) {
	switch s {
	case BROADCAST:
		p.msgService.BroadcastMsg(msg)
	case LEADER:
		p.msgService.SendMsg(p.leader.String(), msg)
	case PEER:
		p.msgService.SendMsg(id, msg)
	}
}

func (p *Pacemaker) AttachMsgLayer(msgService MessagingLayer) {
	p.msgService = msgService
}

func (p *Pacemaker) Start() {
	// senidng NEWEPOCH message after every ∆resend time
	go func() {
		for {
			select {
			case event := <-p.recEventsChan:
				// handle the events
				// add to recEvents array for future processing
				p.recEvents.Add(event)

			case <-p.TimerResend.Subscribe():
				// Broadcast NEWEPOCH message
				payload := &rpc.NewEpochMesage{
					EpochID: p.currEpochStat.e,
				}
				msg, err := rpc.NewMessageBuilder(
					*p.msgService.GetCodec(),
				).SetHeaders(rpc.MessageNewEpoch).SetTopic(rpc.Pacemaker).SetData(payload).Bytes()

				if err != nil {
					panic(err)
				}

				p.msgService.BroadcastMsg(msg)
				p.logger.Info("PACEMAKER: NEWEPOCH message sent")

			case <-p.TimerProgress.Subscribe():
				// if no progress has been made (this can be quanitfied from signals coming from)
				// the reporting engine; then the leader is considered faulty
				// change epoch and suspend current reporting engine

			case <-p.quitCh: // Add a quit channel for graceful shutdown
				p.logger.Info("PACEMAKER: stopped")
				return
			}
		}
	}()

	p.server.peers.Each(func(k string, v *ProtcolPeer) {
		p.logger.Infof("PACEMAKER: peer network id %s , server ID %s , server %s", v.ID, v.ServerID, p.ocrCtx.ID)
	})

	// select a leader
	leader := exponentialBackoff(initTickerTime, maxBackoff, int(p.currEpochStat.e), secretKey, p.server.peers, p.ocrCtx.ID)

	p.logger.Infof("PACEMAKER: Leader is %s , server ID : %v", leader.ServerID, p.ocrCtx.ID)

	var isLeader bool
	if leader.ServerID == p.ocrCtx.ID {
		isLeader = true
	} else {
		isLeader = false
	}

	// update teh ocr staet accordingly
	if isLeader {
		p.upateOCRState(rpc.LEADING)

		cfg := jobs.JobSourceConfig{
			SourceType: jobs.JobSourceDir,
			DirPath:    jobsDir,
		}
		// launch event listeners -- only if ur a leader
		go p.launchJobListeners(cfg)

		// give job listenre some time to read the events on-chain
		// Adaptive waiting for events (instead of fixed 5s sleep)
		timeout := 10 * time.Second
		if !p.waitForEvents(timeout) {
			p.logger.Warn("PACEMAKER: No events received within timeout. Triggering leader change...")
			// new epoch msg to swithc to new leader
			p.SendNewEpochMsg()
		}

	} else {
		p.upateOCRState(rpc.FOLLOWING)
	}

	// p_globals
	p_globals := PacemakerGlobals{
		n: uint(p.currEpochStat.n),
		f: uint(p.currEpochStat.f),
	}

	s_info := ServerInfo{
		Addr:  p.server.ID().Address().String(),
		Codec: p.server.Codec,
		ID:    p.server.id.String(),
	}

	// init the Reporter
	initReporter(p.Reporter, isLeader, p.currEpochStat.e, leader.ServerID, s_info, p_globals, *p.signer, p.server)

	time.Sleep(2 * time.Second)

	// if the node is a follower, p.recEvents should be len(0) and will be ignored
	go p.Reporter.Start(&p.recEvents, &p.jobRegistry)

}

func (p *Pacemaker) ProcessMessage(msg *rpc.DecodedMsg) error {
	switch msg.Data.(type) {
	case *rpc.NewEpochMesage:
		newEMsg := msg.Data.(*rpc.NewEpochMesage)

		p.logger.Infof("PACEMAKER : new epoch msg received.. %+v from %s\n", newEMsg, msg.FromId)
		p.logger.Infof("PACEMAKER : connected node count is : %v\n", p.ocrCtx.PeerCount)
		p.logger.Infof("PACEMAKER : faulty node count is : %v\n", p.ocrCtx.FaultyCount)

		p.processNewEpochMsg(msg)

		return nil
	case rpc.ChangeLeaderMessage:
		// TODO: change leader and launch new epoch
		// idt its needed since new epoch msg achives the same thing

		return nil
	default:
		p.logger.Errorf("PACEMAKER RPC Handler: unkown rpc msg type")
		return fmt.Errorf("PACEMAKER RPC Handler: unkown rpc msg type")
	}
}

func (p *Pacemaker) processNewEpochMsg(msg *rpc.DecodedMsg) {
	newEMsg := msg.Data.(*rpc.NewEpochMesage)
	e0 := newEMsg.EpochID
	f := p.currEpochStat.f

	if e0 > p.currEpochStat.e {
		p.currEpochStat.ne = e0 // update highest recorded e
		p.currEpochStat.n_e0++

		if p.currEpochStat.n_e0 > f {
			newEpoch := uint64(f + 1 - p.currEpochStat.ne)
			p.currEpochStat.new_e = newEpoch
			// broadcast it
			go p.SendNewEpochMsg()
		} else if p.currEpochStat.n_e0 > 2*f {
			// suspend current RE
			// update current epoch to new epoch which is
			// (2f+1) - ne
			// start new RE
			p.switchToNewEpoch()
		}
	}

	// ignore
}

// wrapper on common functions
func (p *Pacemaker) SendNewEpochMsg() {
	// Broadcast NEWEPOCH message
	payload := &rpc.NewEpochMesage{
		EpochID: p.currEpochStat.e,
	}
	msg, err := rpc.NewMessageBuilder(
		*p.msgService.GetCodec(),
	).SetHeaders(rpc.MessageNewEpoch).SetTopic(rpc.Pacemaker).SetData(payload).Bytes()

	if err != nil {
		panic(err)
	}

	p.msgService.BroadcastMsg(msg)
}

func (p *Pacemaker) upateOCRState(s rpc.OCRState) {
	p.logger.Infof("PACEMAKER: updating OCR state to %s, server id %s", s, p.ocrCtx.ID)
	paceMakerMsg := &rpc.PacemakerMessage{
		Data: s,
	}

	p.ocrCh <- paceMakerMsg
}

func (p *Pacemaker) switchToNewEpoch() {
	// update current epoch
	f := p.currEpochStat.f
	p.currEpochStat.e = (2*f + 1) - p.currEpochStat.ne
	p.currEpochStat.n_e0 = 0
	p.currEpochStat.new_e = 0

	var Isleader bool
	// recEvents are the events that have been recorded by the reporting engine during current epoch

	// if leader , send in the recEvents
	// if follower, send in empty queue
	leader := exponentialBackoff(initTickerTime, maxBackoff, int(p.currEpochStat.e), secretKey, p.server.peers, p.ocrCtx.ID)

	if leader.ID == p.ocrCtx.ID {
		Isleader = true
	} else {
		Isleader = false
	}

	if p.Reporter != nil {
		p.Reporter.Stop()
	} else {
		// if not started yet
		pGlobals := PacemakerGlobals{
			n: uint(p.currEpochStat.n),
			f: uint(p.currEpochStat.f),
		}
		initReporter(p.Reporter, Isleader, p.currEpochStat.e, leader.ID, p.Reporter.serverOpts, pGlobals, *p.signer, p.server)
	}

	if leader.ID == p.ocrCtx.ID {
		p.Reporter.Start(&p.recEvents, &p.jobRegistry)
	} else {
		p.Reporter.Start(&fifo.Queue{}, &p.jobRegistry) // send in empty queue if follower
	}

	p.logger.Infof("PACEMAKER: switched to new epoch %v\n", p.currEpochStat.e)
}

// read the jobs from toml specs, launch thier event listeners,
// use a channel to receved the receivedEvents (if any) and store them in some state
// schedule tehm to be sent for observations (schedule is FIFO queue)
// if no receveid Events, change leader msg emit if leader
// only those jobs that have been received are scheduled during next epoch
func (p *Pacemaker) launchJobListeners(config jobs.JobSourceConfig) {
	var readers []io.Reader
	var err error

	// Select appropriate job reader based on config
	switch config.SourceType {
	case jobs.JobSourceDir:
		readers, err = jobs.ReadJobsFromDir(config.DirPath)

	default:
		p.logger.Errorf("RE: Unknown job source type: %v", config.SourceType)
		return
	}

	if err != nil {
		p.logger.Errorf("RE: Error fetching jobs from %v: %v", config.SourceType, err)
		return
	}

	// Process the job readers
	jobReaderFac := jobs.NewJobReaderFactory()
	jobReaderconfig := jobs.JobReaderConfig{
		JobFormat: jobs.JobFormatTOML,
	}

	for _, reader := range readers {
		job, err := jobReaderFac.Read(reader, jobReaderconfig)
		if err != nil {
			p.logger.Errorf("RE: Error creating job from reader: %v", err)
			continue
		}

		// Add job to registry
		p.jobRegistry[job.ID()] = job

		// Launch event listeners
		go job.Listen(p.recEventsChan)
	}
}

func (p *Pacemaker) waitForEvents(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if p.recEvents.Len() > 0 {
			return true // Events detected within timeout
		}
		time.Sleep(500 * time.Millisecond) // Poll every 500ms
	}
	return false // No events within timeout
}

func exponentialBackoff(
	initTickerTime time.Duration,
	maxBackoff time.Duration,
	epoch int,
	secretKey []byte,
	peers *avl.Tree[string, *ProtcolPeer],
	serverID string,
) *ProtcolPeer {
	backoff := initTickerTime
	for {

		logger.Get().Sugar().Info("PACEMAKER: finding leader ", peers.Size(), epoch)
		leader := findLeader(serverID, epoch, secretKey, peers)
		if leader != nil {
			return leader
		}

		logger := logger.Get().Sugar()
		logger.Warnf("Leader selection failed, retrying in %v...", backoff)

		// Wait for the backoff duration
		time.Sleep(backoff)

		// Increase backoff time exponentially, but cap it at maxBackoff
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
