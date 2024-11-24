package ocr

import "eavesdrop/rpc"

type Transmitter interface {
	// TODO: signature of this func is yet to be decided
	Transmit() error
	ProcessMessage(msg *rpc.DecodedMsg) error
}

// implemetns both the Transmitter and rpc.RPCProcessor interface
type MyTransmitter struct {
	// ... fields to be added
	Codec rpc.Codec
}

func (t *MyTransmitter) Transmit() error {
	// ... implementation to be added
	return nil
}

func (t *MyTransmitter) ProcessMessage(msg *rpc.DecodedMsg) error {
	// ... implementation to be added
	return nil
}
