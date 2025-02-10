package ocr

import (
	"crypto"
	"eavesdrop/rpc"
	"time"
)

const (
	MAX_ROUNDS                = 10
	INIT_ROUND                = 1
	RoundTmeout time.Duration = time.Second * 20
	RoundGrace  time.Duration = time.Second * 5
)

type GeneralState struct {
	sentEcho       Report // the echoed attested report (TODO: shoudl be a diff DS)
	sentReport     bool   // indicates if REPORT message has been sent for this round/ attested report which has been sent for this round
	completedRound bool   // indicates if current round is finished
	receivedEcho   []bool // jth element true iff received FINAL - ECHO message with valid attested report from pj
}

type ReportingEngine struct {
	curRound int               // current round (shared state between leader and non-leading oracles)
	isLeader bool              // indicates if the oracle is the leader or follower
	epoch    uint64            // current epoch number
	leader   *crypto.PublicKey // current leader
	GeneralState
	LeaderState // set either of the states, depending upon if hte oracle is leader or not
	msgService  MessagingLayer
	quitCh      chan struct{}
}

func NewReportingEngine(isLeader bool, epoch uint64, leader *crypto.PublicKey) *ReportingEngine {
	re := &ReportingEngine{
		epoch:  epoch,
		leader: leader,
		GeneralState: GeneralState{
			sentEcho:       Report{},
			sentReport:     false,
			completedRound: false,
			receivedEcho:   make([]bool, 1),
		},
		LeaderState: LeaderState{
			observations:      make([]Observation, 1), // init of size 1, we can extend it in future
			reports:           make([]Report, 1),
			TimerRoundTimeout: NewTimer(RoundTmeout),
			TimerGrace:        NewTimer(RoundGrace),
			Phase:             PhaseNil, // should only be set if leader
			phaseCh:           make(chan LeaderPhase),
		},
		curRound: INIT_ROUND,
		isLeader: isLeader,
	}

	// if leader , do some intial config
	if isLeader {
		re.LeaderState.Phase = PhaseObserve // inital state
	}

	return re
}

func (re *ReportingEngine) Start() {
	// function's design:
	// permanet listener, will use a quitch to stop the listener and current RE
	// if leader: will listen to channel for updates to re.phase and upon each phase
	// change, will start a new goroutine to handle the phase, those handlers will then process the rpc messages and update the state and phase as necessaryu
	// which will be then picked up by the main listener
	// if non-leader: will listen to rpc msgs from the leader and handle them accordingly
	// in fact a meta-seprator can be used where if hte re is a leader, launcha main listener in a go routine and then launch a phase handler in a go routine
	// if non-leader, launch a main listener in a separate go routine

	if re.isLeader {
		go re.orchestrateLeadership()
		time.Sleep(1 * time.Second) // to ensure the leader is ready
		re.LeaderState.phaseCh <- PhaseObserve
	} else {
		go re.orchestrateFollowing()
	}

free:
	for {
		select {
		case <-re.quitCh:
			break free
		}
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
		re.msgService.SendMsg("", msg)
	case PEER:
		re.msgService.SendMsg(id, msg)
	}
}

func (re *ReportingEngine) AttachMsgLayer(msgService MessagingLayer) {
	re.msgService = msgService
}

// all messages with say topic Reporter will be routed to repoter.ProcessMessage, now this msgs could be of any type , like leader senidng msg, braodcsating something, an oracle submitting an observation (all of these msgs should be defiend in rpc_leader.go inside rpc pkg i think), so reporter.ProcessMessage could either handle all of them itself and update the state of the RE glovally or it could delegrate, for now, lets shouild build a monolithic RPC handler for reptoer inside ProcessMessgae
func (re *ReportingEngine) ProcessMessage(*rpc.DecodedMsg) error {
	return nil
}

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
