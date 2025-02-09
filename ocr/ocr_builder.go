package ocr

import (
	"eavesdrop/rpc"
	"fmt"
)

type OCRBuilder struct {
	transmitter Transmitter
	codec       rpc.Codec
}

func (b *OCRBuilder) WithTransmitter(transmitter Transmitter) *OCRBuilder {
	b.transmitter = transmitter
	return b
}

func (b *OCRBuilder) WithCodec(codec rpc.Codec) *OCRBuilder {
	b.codec = codec
	return b
}

func (b *OCRBuilder) Build() (*OCR, error) {
	if b.transmitter == nil || b.codec == nil {
		return nil, fmt.Errorf("missing required components")
	}

	return &OCR{
		Codec: b.codec,
	}, nil
}
