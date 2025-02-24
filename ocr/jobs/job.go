package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type JobType string

const (
	DirectReqJob JobType = "directrequest"
	CcipJob      JobType = "ccip"
)

// to be sent over teh network from the leader to followers
// on the account that recipients might not have this particular job
// in their registry and hence would need info to make observations
// sent along OBSERVE-REQ msg
type JobInfo struct {
	JobID    string
	JobType  JobType
	Template interface{}
	Timeout  time.Duration
}

type JobEventResponse interface {
	JobID() string
	Event() string       // event signature
	Result() interface{} // could be a value, string or a struct
}

type Job interface {
	ID() string
	Type() JobType
	Payload() interface{}
	Run(context.Context) error    // inputs, ctx, logger etc... need to be passed
	Result() ([]byte, error)      // run computes teh result and stores it into the job DS and the result is accessible via this func
	TaskTimeout() time.Duration   // timeout for job computation, if not completed by then, the job is killed
	Listen(chan JobEventResponse) // listen for events, if any and update over channel passed as input
	ReportAssembler
}

// NewJobFromInfo creates a concrete job instance from JobInfo
func NewJobFromInfo(info JobInfo) (Job, error) {
	switch info.JobType {
	case DirectReqJob:
		var params DirectRequestTemplateParams
		templateBytes, err := json.Marshal(info.Template) // Convert interface{} to JSON
		if err != nil {
			return nil, fmt.Errorf("failed to marshal template: %v", err)
		}
		if err := json.Unmarshal(templateBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to parse job template: %v", err)
		}

		return &DirectRequest{
			JobID:   info.JobID,
			JobType: DirectReqJob,
			params:  params,
			Results: make(map[string]interface{}),
			timeout: uint(info.Timeout),
			mu:      sync.Mutex{},
		}, nil

	case CcipJob:
		// Future CCIP job implementation
		return nil, errors.New("CCIP job implementation not provided yet")

	default:
		return nil, errors.New("unknown job type")
	}
}
