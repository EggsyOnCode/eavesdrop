package ocr

import (
	"eavesdrop/rpc"
	"eavesdrop/utils"
)

type LeaderPhase byte

const (
	PhaseNil     LeaderPhase = 0xff
	PhaseObserve LeaderPhase = 0x0
	PhaseGrace   LeaderPhase = 0x1
	PhaseReport  LeaderPhase = 0x2
	PhaseFinal   LeaderPhase = 0x3
)

type Observation struct{}

type LeaderState struct {
	observations      []Observation // signed observations received in OBSERVE messages
	reports           []Report      // attested reports received in REPORT messages
	TimerRoundTimeout *Timer        // timer Tround with timeout duration ∆round , initially stopped
	TimerGrace        *Timer        // timer Tgrace with timeout duration ∆grace , initially stopped
	Phase             LeaderPhase   // current phase of the leader
	phaseCh           chan LeaderPhase
}

func (re *ReportingEngine) handleObserve() {
	// handle the OBSERVE phase
	// will have to listen to the OBSERVE messages from the leader
	// and update the state accordingly

	// increment hte round number
	re.curRound++

	// send out OBSERVE-REQ msgs and init the observations and reports if not already done
	observeReqMsg := rpc.ObserveReq{
		Epoch:  re.epoch,
		Round:  uint64(re.curRound),
		Leader: re.leader,
	}

	// constructing msg
	rpcMsg, err := rpc.NewRPCMessageBuilder(
		utils.NetAddr(re.serverOpts.Addr),
		re.serverOpts.Codec,
		re.serverOpts.ID,
	).SetHeaders(
		rpc.MessageObserveReq,
	).SetTopic(
		rpc.Reporter,
	).SetPayload(observeReqMsg).Bytes()

	if err != nil {
		panic(err)
	}

	re.msgService.BroadcastMsg(rpcMsg)

	// start the round timer (have a handler for the timer expiry) in top most loop
	re.LeaderState.TimerRoundTimeout = NewTimer(RoundTmeout)

	// change state to OBSERVE
	re.Phase = PhaseObserve
	re.phaseCh <- PhaseObserve
}

func (re *ReportingEngine) handleGrace() {
	// handle the GRACE phase
	// will have to listen to the GRACE messages from the leader
	// and update the state accordingly
}

func (re *ReportingEngine) handleReport() {
	// handle the REPORT phase
	// will have to listen to the REPORT messages from the leader
	// and update the state accordingly
}

func (re *ReportingEngine) handleFinal() {
	// handle the FINAL phase
	// will have to listen to the FINAL messages from the leader
	// and update the state accordingly
}
