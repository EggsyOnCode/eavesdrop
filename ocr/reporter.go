package ocr

import "eavesdrop/rpc"

type Reporter interface {
	// TODO: signature of this func is yet to be decided
	Report() error
	ProcessMessage(msg *rpc.DecodedMsg) error
}

// implemetns both the Reporter and rpc.RPCProcessor interface
type MyReporter struct {
	// ... fields to be added
	Codec rpc.Codec
}

func (r *MyReporter) Report() error {
	// ... implementation to be added
	return nil
}

func (r *MyReporter) ProcessMessage(msg *rpc.DecodedMsg) error {
	// ... implementation to be added
	return nil
}
