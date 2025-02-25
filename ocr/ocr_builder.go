package ocr

import (
	"eavesdrop/rpc"
	"fmt"
)

type OCRBuilder struct {
	codec       rpc.Codec
}

func (b *OCRBuilder) WithCodec(codec rpc.Codec) *OCRBuilder {
	b.codec = codec
	return b
}

func (b *OCRBuilder) Build() (*OCR, error) {
	if b.codec == nil {
		return nil, fmt.Errorf("missing required components")
	}

	return &OCR{
		Codec: b.codec,
	}, nil
}
