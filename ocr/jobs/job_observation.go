package jobs


type JobObservationResponse struct {
	JobId    string `json:"job_id"`
	Response []byte `json:"response"`
}