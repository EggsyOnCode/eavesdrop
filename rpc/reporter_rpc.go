package rpc

import "eavesdrop/ocr/jobs"

// collection of rpc messages for Reporter Protocol instance
type ObserveReq struct {
	Epoch  uint64         `json:"epoch"`
	Round  uint64         `json:"round"`
	Leader string         `json:"leader"`
	Jobs   []jobs.JobInfo `json:"jobs"`
}

func (o *ObserveReq) Bytes(c Codec) ([]byte, error) {
	return c.Encode(o)
}

type ObserveResp struct {
	Epoch  uint64 `json:"epoch"`
	Round  uint64 `json:"round"`
	Leader string `json:"leader"`

	JobResponses []jobs.JobObservationResponse `json:"jobResponses"`
}

func (o *ObserveResp) Bytes(c Codec) ([]byte, error) {
	return c.Encode(o)
}

type ReportReq struct {
	Epoch  uint64 `json:"epoch"`
	Round  uint64 `json:"round"`
	Leader string `json:"leader"`

	Observations []byte `json:"observations"` // would be ObservationMap serialized into JSON
}

func (rr *ReportReq) Bytes(c Codec) ([]byte, error) {
	return c.Encode(rr)
}

type JobReports map[string]interface{}

type ReportRes struct {
	Epoch   uint64     `json:"epoch"`
	Round   uint64     `json:"round"`
	Leader  string     `json:"leader"`
	Reports JobReports `json:"reports"`
}

func (rr *ReportRes) Bytes(c Codec) ([]byte, error) {
	return c.Encode(rr)
}

// dessimination of ObservationMap for current round
type BroadcastObservationMap struct {
	Epoch        uint64 `json:"epoch"`
	Round        uint64 `json:"round"`
	Leader       string `json:"leader"`
	Observations []byte `json:"observations"` // would be ObservationMap serialized into JSON
}

func (om *BroadcastObservationMap) Bytes(c Codec) ([]byte, error) {
	return c.Encode(om)
}