package ocr

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"eavesdrop/ocr/jobs"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"fmt"
	"reflect"
	"time"

	fifo "github.com/foize/go.fifo"

	"go.uber.org/zap"
)

const (
	MAX_ROUNDS                = 10
	INIT_ROUND                = 1
	RoundTmeout time.Duration = time.Second * 20
	RoundGrace  time.Duration = time.Second * 5

	jobsDir string = "../jobs_def"
)

// info about the server (p2p network) that needs to be exposed to the reporting engine
type ServerInfo struct {
	Addr  string
	Codec rpc.Codec
	ID    string
}

// some globals from Pacemaker
type PacemakerGlobals struct {
	n uint // total number of oracles
	f uint // number of faulty oracles
}

// lasts only for a round and is reset after that
type CacheLayer struct {
	observe_n uint // no of observations received
	report_n  uint // no of reports received
}

type GeneralState struct {
	sentEcho       Report // the echoed attested report (TODO: shoudl be a diff DS)
	sentReport     bool   // indicates if REPORT message has been sent for this round/ attested report which has been sent for this round
	completedRound bool   // indicates if current round is finished
	receivedEcho   []bool // jth element true iff received FINAL - ECHO message with valid attested report from pj
}

type ReportingEngine struct {
	curRound int    // current round (shared state between leader and non-leading oracles)
	isLeader bool   // indicates if the oracle is the leader or follower
	epoch    uint64 // current epoch number
	leader   string // current leader's PeerID
	GeneralState
	LeaderState      // set either of the states, depending upon if hte oracle is leader or not
	msgService       MessagingLayer
	serverOpts       ServerInfo
	quitCh           chan struct{}
	logger           *zap.SugaredLogger
	cacheLayer       CacheLayer
	pacemakerGlobals PacemakerGlobals
	signer           crypto.PrivateKey
	recEvents        *fifo.Queue
	jobRegistry      *map[string]jobs.Job // used to query the job info when an event is received
	jobSchedule      map[int][]jobs.Job   // schedule of jobs to be observed in each round
}

func NewReportingEngine(isLeader bool, epoch uint64, leader string, s_info ServerInfo, p_globals PacemakerGlobals, signer ecdsa.PrivateKey) *ReportingEngine {
	re := &ReportingEngine{
		epoch:       epoch,
		serverOpts:  s_info,
		leader:      leader,
		jobSchedule: map[int][]jobs.Job{},
		signer:      signer,
		jobRegistry: &map[string]jobs.Job{},
		GeneralState: GeneralState{
			sentEcho:       Report{},
			sentReport:     false,
			completedRound: false,
			receivedEcho:   make([]bool, 1),
		},
		LeaderState: LeaderState{
			observations: map[string]Observation{}, // init of size 1, we can extend it in future
			reports:      make([]Report, 1),
			// TimerRoundTimeout: NewTimer(RoundTmeout),
			TimerRoundTimeout: &Timer{},
			// TimerGrace:        NewTimer(RoundGrace),
			TimerGrace: &Timer{},
			Phase:      PhaseNil, // should only be set if leader
			phaseCh:    make(chan LeaderPhase),
		},
		curRound:         INIT_ROUND,
		isLeader:         isLeader,
		pacemakerGlobals: p_globals,
		cacheLayer: CacheLayer{
			observe_n: 0,
			report_n:  0,
		},
	}

	// config logger setup
	re.logger = zap.S().With("epoch", epoch, "leader", leader, "round", re.curRound)

	// if leader , do some intial config
	if isLeader {
		re.LeaderState.Phase = PhaseObserve // inital state
	}

	return re
}

func (re *ReportingEngine) Start(recEvents *fifo.Queue, jobReg *map[string]jobs.Job) {
	// function's design:
	// permanet listener, will use a quitch to stop the listener and current RE
	// if leader: will listen to channel for updates to re.phase and upon each phase
	// change, will start a new goroutine to handle the phase, those handlers will then process the rpc messages and update the state and phase as necessaryu
	// which will be then picked up by the main listener
	// if non-leader: will listen to rpc msgs from the leader and handle them accordingly
	// in fact a meta-seprator can be used where if hte re is a leader, launcha main listener in a go routine and then launch a phase handler in a go routine
	// if non-leader, launch a main listener in a separate go routine
	re.jobRegistry = jobReg

	if re.isLeader {

		// schedule the jobs to be observed during each round of the epoch
		re.recEvents = recEvents // received from pacemaker
		n := recEvents.Len()
		jobs_per_round := (n / MAX_ROUNDS)
		for i := 0; i < MAX_ROUNDS; i++ {
			for j := 0; j < int(jobs_per_round); j++ {
				job := recEvents.Next()
				re.jobSchedule[i] = append(re.jobSchedule[i], job.(jobs.Job))
			}
		}

		go re.orchestrateLeadership()
		time.Sleep(1 * time.Second) // to ensure the leader is ready
		re.LeaderState.phaseCh <- PhaseObserve
	} else {
		go re.orchestrateFollowing()
	}
}

func (re *ReportingEngine) orchestrateLeadership() {
	// leader orchestration
	// will have to listen to the phase changes and then start a new goroutine to handle the phase
	// the phase handlers will then process the rpc messages and update the state and phase as necessary
	// which will be then picked up by the main listener

free:
	for {
		select {
		case phase := <-re.LeaderState.phaseCh:
			switch phase {
			case PhaseObserve:
				go re.handleObserve()
			case PhaseGrace:
				go re.handleGrace()
			case PhaseReport:
				go re.handleReport()
			case PhaseFinal:
				go re.handleFinal()
			}

		case <-re.quitCh:
			break free
		}
	}
}

func (re *ReportingEngine) orchestrateFollowing() {
	// non-leader orchestration
	// will listen to rpc msgs from the leader and handle them accordingly

free:
	for {
		select {
		case <-re.quitCh:
			break free
		}
	}
}

// msg expected is serialized version of RPCMessage
func (re *ReportingEngine) SendMsg(s SendingSchme, msg []byte, id string) {
	switch s {
	case BROADCAST:
		re.msgService.BroadcastMsg(msg)
	case LEADER:
		// will have to fetch leader info from Packemaker via channels
		re.msgService.SendMsg(re.leader, msg)
	case PEER:
		re.msgService.SendMsg(id, msg)
	}
}

func (re *ReportingEngine) AttachMsgLayer(msgService MessagingLayer) {
	re.msgService = msgService
}

// all messages with say topic Reporter will be routed to repoter.ProcessMessage, now this msgs could be of any type , like leader senidng msg, braodcsating something, an oracle submitting an observation (all of these msgs should be defiend in rpc_leader.go inside rpc pkg i think), so reporter.ProcessMessage could either handle all of them itself and update the state of the RE glovally or it could delegrate, for now, lets shouild build a monolithic RPC handler for reptoer inside ProcessMessgae
func (re *ReportingEngine) ProcessMessage(msg *rpc.DecodedMsg) error {
	switch msg.Data.(type) {
	case rpc.ObserveReq:
		msg := msg.Data.(rpc.ObserveReq)
		re.logger.Infof("RE: received OBSERVE-REQ msg: %v", msg)

		re.curRound = int(msg.Round)
		if re.curRound > MAX_ROUNDS {
			re.logger.Infof("RE: max rounds reached, stopping RE")
			// broadcast changeleader event

			changeLeaderMsg := rpc.ChangeLeaderMessage{}

			// constructing msg
			rpcMsg, err := rpc.NewRPCMessageBuilder(
				utils.NetAddr(re.serverOpts.Addr),
				re.serverOpts.Codec,
				re.serverOpts.ID,
			).SetHeaders(
				rpc.MessageChangeLeader,
			).SetTopic(
				rpc.Pacemaker,
			).SetPayload(changeLeaderMsg).Bytes()

			if err != nil {
				panic(err)
			}

			re.msgService.BroadcastMsg(rpcMsg)

			re.Stop()
		}

		// make observation about the given jobId
		res := re.observe("dummy_jobid")

		observeResmsg := rpc.ObserveResp{
			Epoch:    re.epoch,
			Leader:   re.leader,
			Round:    uint64(re.curRound),
			Response: res,
		}

		// signing observeMsg
		msgBytes, err := observeResmsg.Bytes(re.serverOpts.Codec)
		if err != nil {
			re.logger.Errorf("RE: err converting to bytes")
			return nil
		}

		signature, err := re.SignMessage(msgBytes)
		if err != nil {
			re.logger.Errorf("RE: err signing msg bytes")
			return nil
		}

		// constructing msg
		rpcMsg, err := rpc.NewRPCMessageBuilder(
			utils.NetAddr(re.serverOpts.Addr),
			re.serverOpts.Codec,
			re.serverOpts.ID,
		).SetHeaders(
			rpc.MessageObserveRes,
		).SetTopic(
			rpc.Reporter,
		).SetPayload(observeResmsg).SetSignature(signature).Bytes()

		if err != nil {
			panic(err)
		}

		re.msgService.BroadcastMsg(rpcMsg)

		return nil

	case rpc.ObserveResp:
		observation := msg.Data.(rpc.ObserveResp)
		if (observation.Round == uint64(re.curRound)) && (observation.Epoch == re.epoch) {
			// update the cache layer
			re.cacheLayer.observe_n++
			// add observations to the state
			re.observations[msg.FromId] = observation.Response

			if re.cacheLayer.observe_n > 2*re.pacemakerGlobals.f+1 {
				re.Phase = PhaseGrace
				re.LeaderState.phaseCh <- PhaseGrace
			}
		}

		return nil

	default:
		re.logger.Errorf("RE: unknown message type: %v", reflect.TypeOf(msg.Data))
		return nil
	}
}

// to be called by the pacemaker, should return the imp state of current
// epoch needed to bootstrap the new epoch
func (r *ReportingEngine) Stop() {
	// stop the reporting engine
	close(r.quitCh)
	// cleanup the state
	r.cleanupFunc()
}

// TODO: implement this
func (re *ReportingEngine) cleanupFunc() {
	// cleanup function
	// will be called when the reporting engine is stopped
	// will have to cleanup the state and any other resources
}

// for makign observations about a specific jobId
func (re *ReportingEngine) observe(jobId string) []byte {
	// to be implemented
	return []byte("hello")
}

func (re *ReportingEngine) SignMessage(msg []byte) ([]byte, error) {
	// Assert that re.signer is an ECDSA private key
	privKey, ok := re.signer.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("signer is not an ECDSA private key")
	}

	// Hash the message
	hashed := sha256.Sum256(msg)

	// Sign the message
	r, s, err := ecdsa.Sign(rand.Reader, privKey, hashed[:])
	if err != nil {
		return nil, err
	}

	// Convert r and s to a byte slice (you may need to serialize this differently)
	signature := append(r.Bytes(), s.Bytes()...)
	return signature, nil
}
