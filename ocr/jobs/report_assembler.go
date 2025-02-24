package jobs

import "fmt"

type ReportAssembler interface {
	// Assmble returns a byte arry, how we cast it back to a definite type depends on the return type
	// specified in the job spec
	Assemble(observations []JobObservationResponse) ([]byte, error)
}

func NewReportAssembler(strategy ReportingStrategy) (ReportAssembler, error) {
	switch strategy {
	case "mean":
		return &MeanValueAssembler{}, nil
	default:
		return nil, fmt.Errorf("unsupported strategy: %s", strategy)
	}
}

type MeanValueAssembler struct{}

func (mva *MeanValueAssembler) Assemble(obs []JobObservationResponse) ([]byte, error) {
	return nil, nil
}
