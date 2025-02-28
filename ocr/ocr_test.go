package ocr

import (
	"eavesdrop/crypto"
	"eavesdrop/rpc"
	"testing"
	"time"
)

func TestOCR(t *testing.T) {
	s1Opts := &ServerOpts{
		// ListenAddr: "127.0.0.1:3000",
		CodecType:  rpc.JsonCodec,
		PrivateKey: crypto.GeneratePrivateKey(),
	}

	s1 := NewServer(s1Opts)

	s2Opts := &ServerOpts{
		// ListenAddr: "127.0.0.1:4000",
		CodecType:  rpc.JsonCodec,
		PrivateKey: crypto.GeneratePrivateKey(),
	}

	s2 := NewServer(s2Opts)

	s3Opts := &ServerOpts{
		// ListenAddr: "127.0.0.1:4000",
		CodecType:  rpc.JsonCodec,
		PrivateKey: crypto.GeneratePrivateKey(),
	}

	s3 := NewServer(s3Opts)

	time.Sleep(2 * time.Second)

	ocr1 := NewOCR(s1)
	ocr2 := NewOCR(s2)
	ocr3 := NewOCR(s3)

	go ocr1.Start()
	go ocr2.Start()
	go ocr3.Start()

	select {}
}
