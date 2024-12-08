package ocr

import "eavesdrop/rpc"

const (
	MAX_ROUNDS = 10
)

type GeneralState struct {
	currRound      int  // current round for non-leading oracles
	sentEcho       bool // echoed attested report which has been sent for this round
	sentReport     bool // indicates if REPORT message has been sent for this round/ attested report which has been sent for this round
	completedRound bool // indicates if current round is finished
	receivedEcho   []bool
}

type LeaderState struct {
	currRound         int           // current round for leader
	observations      []Observation // signed observations received in OBSERVE messages
	reports           []Report      // attested reports received in REPORT messages
	TimerRoundTimeout *Timer        // timer Tround with timeout duration ∆round , initially stopped
	TimerGrace        *Timer        // timer Tgrace with timeout duration ∆grace , initially stopped
}

type ReportingEngine struct {
	GeneralState
	LeaderState // set either of the states, depending upon if hte oracle is leader or not
	msgService  MessagingLayer
}

func NewReportingEngine() *ReportingEngine {
	return &ReportingEngine{
		GeneralState: GeneralState{
			currRound:      0,
			sentEcho:       false,
			sentReport:     false,
			completedRound: false,
			receivedEcho:   make([]bool, MAX_ROUNDS),
		},
		LeaderState: LeaderState{
			currRound:         0,
			observations:      make([]Observation, MAX_ROUNDS),
			reports:           make([]Report, MAX_ROUNDS),
			TimerRoundTimeout: NewTimer(10),
			TimerGrace:        NewTimer(10),
		},
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

func (r *ReportingEngine) AttachMsgLayer(msgService MessagingLayer) {
	r.msgService = msgService
}

func (r *ReportingEngine) Start() {
	// start the reporting engine
}

func (r *ReportingEngine) ProcessMessage(*rpc.DecodedMsg) error {
	return nil
}
