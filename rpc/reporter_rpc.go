package rpc

import "eavesdrop/ocr/jobs"

// collection of rpc messages for Reporter Protocol instance
type ObserveReq struct {
	Epoch  uint64         `json:"epoch"`
	Round  uint64         `json:"round"`
	Leader string         `json:"leader"`
	Jobs   []jobs.JobInfo `json:"jobs"`
}

type JobObservationResponse struct {
	JobId    string `json:"job_id"`
	Response []byte `json:"response"`
}

func (o *ObserveReq) Bytes(c Codec) ([]byte, error) {
	return c.Encode(o)
}

type ObserveResp struct {
	Epoch  uint64 `json:"epoch"`
	Round  uint64 `json:"round"`
	Leader string `json:"leader"`

	JobResponses []JobObservationResponse `json:"jobResponses"`
}

func (o *ObserveResp) Bytes(c Codec) ([]byte, error) {
	return c.Encode(o)
}
