package rpc

// collection of rpc messages for Reporter Protocol instance

type ObserveReq struct {
	Epoch  uint64
	Round  uint64
	Leader string
}

func (o *ObserveReq) Bytes(c Codec) ([]byte, error) {
	return c.Encode(o)
}

type ObserveResp struct {
	Epoch  uint64
	Round  uint64
	Leader string

	Response []byte // this will most likeyl change, maybe the response object can be defind by the jobID itself  
}

func (o *ObserveResp) Bytes(c Codec) ([]byte, error) {
	return c.Encode(o)
}


