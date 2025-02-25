package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
)

type ReportAssembler interface {
	// Assmble returns a any value, its upon the caller to interpret the final val based on job's spec, because regardless, JobReports in map of string -> inteface{}
	// how we cast it back to a definite type depends on the return type
	// specified in the job spec
	Assemble(observations []JobObservationResponse) (interface{}, error)
}

func NewReportAssembler(strategy ReportingStrategy, job Job) (ReportAssembler, error) {
	switch strategy {
	case "mean":
		return &MeanValueAssembler{
			j: job,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported strategy: %s", strategy)
	}
}

type MeanValueAssembler struct {
	j Job
}

func (mva *MeanValueAssembler) Assemble(obs []JobObservationResponse) (interface{}, error) {
	switch mva.j.Type() {
	case DirectReqJob:
		payload := mva.j.Payload().(DirectRequestTemplateParams)

		if payload.ReportingStrategy.ResultType != "numeric" {
			return nil, errors.New("mean can't be calculated on non-numeric data types")
		}

		var sum float64
		var count int

		for _, res := range obs {
			var val float64
			if err := json.Unmarshal(res.Response, &val); err != nil {
				return nil, fmt.Errorf("failed to parse response as float64: %v", err)
			}
			sum += val
			count++
		}

		if count == 0 {
			return nil, errors.New("no valid numeric observations")
		}

		mean := sum / float64(count)

		// no need to Marsahll back into json since the
		// result will be processed locally

		// result, err := json.Marshal(mean)
		// if err != nil {
		// 	return nil, fmt.Errorf("failed to serialize mean result: %v", err)
		// }

		return mean, nil

	default:
		return nil, errors.New("unsupported job type")
	}
}
