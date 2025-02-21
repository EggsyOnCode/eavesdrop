package ocr

import (
	"eavesdrop/crypto"
	"eavesdrop/logger"
	"eavesdrop/ocr/jobs"
	"eavesdrop/rpc"
	"time"

	fifo "github.com/foize/go.fifo"
	"go.uber.org/zap"
)

type SendingSchme int

const (
	BROADCAST SendingSchme = 0
	LEADER    SendingSchme = 1
	PEER      SendingSchme = 2

	ResendTimes  time.Duration = time.Second * 3
	ProgressTime time.Duration = time.Second * 20
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

	// select a leader
	leader := findLeader(int(p.currEpochStat.e), secretKey, p.server.peers)
	// update teh ocr staet accordingly
	if leader.ID == p.ocrCtx.ID {
		p.upateOCRState(rpc.LEADING)
	} else {
		p.upateOCRState(rpc.FOLLOWING)
	}

	// launch event listeners
	go p.launchJobListeners()
}

func (p *Pacemaker) ProcessMessage(msg *rpc.DecodedMsg) error {
	switch msg.Data.(type) {
	case *rpc.NewEpochMesage:
		newEMsg := msg.Data.(*rpc.NewEpochMesage)

		p.logger.Infof("PACEMAKER : new epoch msg received.. %+v from %s\n", newEMsg, msg.FromId)
		p.logger.Infof("PACEMAKER : connected node count is : %v\n", p.ocrCtx.PeerCount)
		p.logger.Infof("PACEMAKER : faulty node count is : %v\n", p.ocrCtx.FaultyCount)

		p.processNewEpochMsg(msg)
	default:
		p.logger.Errorf("PACEMAKER RPC Handler: unkown rpc msg type")
	}

	return nil
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

	// recEvents are the events that have been recorded by the reporting engine during current epoch
	p.Reporter.Stop()

	// if leader , send in the recEvents
	// if follower, send in empty queue
	leader := findLeader(int(p.currEpochStat.e), secretKey, p.server.peers)
	if leader.ID == p.ocrCtx.ID {
		p.Reporter.Start(&p.recEvents, &p.jobRegistry)
	} else {
		p.Reporter.Start(&fifo.Queue{}, &p.jobRegistry) // send in empty queue if follower
	}

	p.logger.Infof("PACEMAKER: switched to new epoch %v\n", p.currEpochStat.e)
}

func (p *Pacemaker) launchJobListeners() {
	// TODO: read the jobs from toml specs, launch thier event listeners,
	// use a channel to receved the receivedEvents (if any) and store them in some state
	// schedule tehm to be sent for observations (schedule is FIFO queue)
	// if no receveid Events, change leader msg emit if leader

	// read jobs
	readers, err := jobs.ReadJobsFromDir(jobsDir)
	if err != nil {
		p.logger.Errorf("RE: err reading jobs from dir: %v", err)
		return
	}

	// create JobStructures
	jobReaderFac := jobs.NewJobReaderFactory()
	jobReaderconfig := jobs.JobReaderConfig{
		JobFormat: jobs.JobFormatTOML,
	}

	for _, reader := range readers {
		job, err := jobReaderFac.Read(reader, jobReaderconfig)
		if err != nil {
			p.logger.Errorf("RE: err creating job from reader: %v", err)
			continue
		}

		// add job to registry
		p.jobRegistry[job.ID()] = job

		// launch event listeners
		go job.Listen(p.recEventsChan)
	}

}
