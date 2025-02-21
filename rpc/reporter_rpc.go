package rpc

import "eavesdrop/ocr/jobs"

// collection of rpc messages for Reporter Protocol instance
type ObserveReq struct {
	Epoch  uint64
	Round  uint64
	Leader string
	Jobs   []jobs.JobInfo
}

type JobObservationResponse struct {
	JobId    string
	Response []byte
}

func (o *ObserveReq) Bytes(c Codec) ([]byte, error) {
	return c.Encode(o)
}

type ObserveResp struct {
	Epoch  uint64
	Round  uint64
	Leader string

	JobResponses []JobObservationResponse
}

func (o *ObserveResp) Bytes(c Codec) ([]byte, error) {
	return c.Encode(o)
}
