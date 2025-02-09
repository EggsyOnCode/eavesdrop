package rpc

import (
	"bytes"
	"encoding/gob"
)

// these constrcuts are for internal async comms between pacemaker and OCR

type OCRState byte

const (
	PREPARE OCRState = 0x0
	FOLLOWING OCRState = 0x1
	LEADING OCRState = 0x2
	REPORT_GEN OCRState = 0x3
	TRANSMIT OCRState = 0x4
)

type PacemakerMessage struct {
	Data    interface{} // OCRStatechange ....
}

type OCRStateChange struct {
	State OCRState
}

func (m *PacemakerMessage) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(m); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}



