package ocr

import (
	"eavesdrop/rpc"
	"fmt"
)

type OCR struct {
	Observer    Observer
	Reporter    Reporter
	Transmitter Transmitter
	Codec       rpc.Codec
}

type OCRBuilder struct {
	observer    Observer
	reporter    Reporter
	transmitter Transmitter
	codec       rpc.Codec
}

func (b *OCRBuilder) WithObserver(observer Observer) *OCRBuilder {
	b.observer = observer
	return b
}

func (b *OCRBuilder) WithReporter(reporter Reporter) *OCRBuilder {
	b.reporter = reporter
	return b
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
	if b.observer == nil || b.reporter == nil || b.transmitter == nil || b.codec == nil {
		return nil, fmt.Errorf("missing required components")
	}

	return &OCR{
		Observer:    b.observer,
		Reporter:    b.reporter,
		Transmitter: b.transmitter,
		Codec:       b.codec,
	}, nil
}
