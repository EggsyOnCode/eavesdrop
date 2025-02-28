package main

import (
	"eavesdrop/crypto"
	"eavesdrop/ocr"
	"eavesdrop/rpc"
	"fmt"
	"log"
	"time"
)

type PeerInfo struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	ListenAddr string `json:"listenAddr"`
}

func main() {
	// gen_keys()
	// The base64-encoded private key string
	// pk1, err := crypto.HexToPrivKey("96ec582ce4da3ac668a5b7b6a44204d9d469673a9f03d1ae16179b938f78acdb")
	// if err != nil {
	// 	log.Fatalf("Failed to decode private key: %v", err)
	// }

	// libp2p1 := network.NewLibp2pTransport(nil, nil, nil, nil)
	// libp2p2 := network.NewLibp2pTransport(nil, nil, nil, nil)
	// go libp2p1.Start()
	// time.Sleep(2 * time.Second)
	// go libp2p2.Start()

	// time.Sleep(4 * time.Second)

	s1Opts := &ocr.ServerOpts{
		// ListenAddr: "127.0.0.1:3000",
		CodecType:  rpc.JsonCodec,
		PrivateKey: crypto.GeneratePrivateKey(),
	}

	s1 := ocr.NewServer(s1Opts)

	s2Opts := &ocr.ServerOpts{
		// ListenAddr: "127.0.0.1:4000",
		CodecType:  rpc.JsonCodec,
		PrivateKey: crypto.GeneratePrivateKey(),
	}

	s2 := ocr.NewServer(s2Opts)

	s3Opts := &ocr.ServerOpts{
		// ListenAddr: "127.0.0.1:4000",
		CodecType:  rpc.JsonCodec,
		PrivateKey: crypto.GeneratePrivateKey(),
	}

	s3 := ocr.NewServer(s3Opts)

	time.Sleep(2 * time.Second)

	ocr1 := ocr.NewOCR(s1)
	ocr2 := ocr.NewOCR(s2)
	ocr3 := ocr.NewOCR(s3)

	go ocr1.Start()
	go ocr2.Start()
	go ocr3.Start()

	select {}
}

func gen_keys() {
	err := crypto.GenerateKeyPairsJSON(2, "peers.json")
	if err != nil {
		log.Fatalf("Error generating key pairs: %v", err)
	}

	fmt.Println("Key pairs generated successfully and saved to keys.json")
}
