package ocr

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
