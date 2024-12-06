package ocr

import "eavesdrop/rpc"

type ObserverRegistry struct {
	Observers map[string]Observer // i.e  keys are JobIDs
}

type Observer interface {
	// Observe sends the observed event to the read-only channel.
	Observe(chan<- interface{}) error
	ProcessMessage(msg *rpc.DecodedMsg) error
}

// implemetns both the Observer and rpc.RPCProcessor interface
type MyObserver struct {
	// ... fields to be added
	Codec rpc.Codec
}

func (o *MyObserver) Observe(ch chan<- interface{}) error {
	// ... implementation to be added
	return nil
}

func (o *MyObserver) ProcessMessage(msg *rpc.DecodedMsg) error {
	// ... implementation to be added
	return nil
}
