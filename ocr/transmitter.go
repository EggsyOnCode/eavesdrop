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
// data is the value to be transmitted
func (t *Transmitter) Transmit(done chan<- bool, data interface{}) {
	l := logger.Get().Sugar()

	l.Infof("TE: tranmitting value to on-chain contract")
	// signal completion

	// switch t.job.Type() {
	// case jobs.DirectReqJob:
	// 	l.Infof("TE: direct request job")

	// 	params := t.job.Payload().(jobs.DirectRequestTemplateParams)
	// 	if params.ChainID == "" {
	// 		l.Errorf("TE: nil params")
	// 		return
	// 	}

	// 	t.execDirectTx(params, data)

	// default:
	// 	l.Errorf("TE: unknown job type")
	// }

	// TODO: add true to the channel to signal job completion
	done <- true
}

func (t *Transmitter) execDirectTx(params jobs.DirectRequestTemplateParams, data interface{}) {
	l := logger.Get().Sugar()

	l.Infof("TE: executing transaction")

	switch params.ChainID {
	case jobs.ChainIDEthereum:

		// TODO: delegate tx building and submission to the TxBuilder class
		// in constructor, it shall take params and data
		// EtehreumTxBuilder and SolanaTxBuilder shall implement the TxBuilder interface
		// and provide the implementation for the BuildTx and SubmitTx methods

	case jobs.ChainIDSolana:

	default:
		l.Errorf("TE: unknown chain ID")
	}
}
