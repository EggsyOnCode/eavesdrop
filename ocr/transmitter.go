package ocr

import (
	"eavesdrop/logger"
	"eavesdrop/ocr/jobs"
)

// one tranmistter instance for each job
type Transmitter struct {
	epoch  uint64
	round  uint64
	leader string
	job    jobs.Job
}

func NewTransmitter(e, r uint64, l string, j jobs.Job) *Transmitter {
	return &Transmitter{
		epoch:  e,
		round:  r,
		leader: l,
		job:    j,
	}
}

// write-only channel
func (t *Transmitter) Transmit(done chan<- bool) {
	l := logger.Get().Sugar()

	l.Infof("TE: tranmitting value to on-chain contract")
	// signal completion
	done <- true
}
