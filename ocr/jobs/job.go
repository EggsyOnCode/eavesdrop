package jobs

import "time"

type JobType string

const (
	DirectReqJob JobType = "directrequest"
	CcipJob      JobType = "ccip"
)

type JobEventResponse interface {
	JobID() string
	Event() string       // event signature
	Result() interface{} // could be a value, string or a struct
}

type Job interface {
	ID() string
	Type() JobType
	Payload() interface{}
	Run() error                   // inputs, ctx, logger etc... need to be passed
	Result() ([]byte, error)      // run computes teh result and stores it into the job DS and the result is accessible via this func
	TaskTimeout() time.Duration   // timeout for job computation, if not completed by then, the job is killed
	Listen(chan JobEventResponse) // listen for events, if any and update over channel passed as input
}
