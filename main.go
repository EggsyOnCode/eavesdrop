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

	// s1Opts := &ocr.ServerOpts{
	// 	ListenAddr: "127.0.0.1:4001",
	// 	CodecType:  rpc.JsonCodec,
	// 	PrivateKey: pk1,
	// }

	// s1 := ocr.NewServer(s1Opts)

	// pk2, err := crypto.HexToPrivKey("c08041b8055826f19b8fda00b929ba17b517a736cf37beaf504c11db77d356f0")
	// if err != nil {
	// 	log.Fatalf("Failed to decode private key: %v", err)
	// }

	// s2Opts := &ocr.ServerOpts{
	// 	ListenAddr: "127.0.0.1:4002",
	// 	CodecType:  rpc.JsonCodec,
	// 	PrivateKey: pk2,
	// }

	// s2 := ocr.NewServer(s2Opts)

	// // ocr1
	// p1 := ocr.NewPaceMaker()
	// p2 := ocr.NewPaceMaker()

	// r1 := ocr.NewReportingEngine()
	// r2 := ocr.NewReportingEngine()

	// ocr1 := ocr.NewOCR(s1, r1, p1)
	// ocr2 := ocr.NewOCR(s2, r2, p2)

	// go ocr1.Start() // s1 would also be init rn
	// go ocr2.Start() // s2 would also be init rn

	// time.Sleep(1 * time.Second)

	// defer ocr1.Stop()
	// defer ocr2.Stop()

	s1Opts := &ocr.ServerOpts{
		// ListenAddr: "127.0.0.1:3000",
		CodecType:  rpc.JsonCodec,
		PrivateKey: crypto.GeneratePrivateKey(),
	}

	s1 := ocr.NewServer(s1Opts)
	go s1.Start()

	s2Opts := &ocr.ServerOpts{
		// ListenAddr: "127.0.0.1:4000",
		CodecType:  rpc.JsonCodec,
		PrivateKey: crypto.GeneratePrivateKey(),
	}

	s2 := ocr.NewServer(s2Opts)
	go s2.Start()

	time.Sleep(2 * time.Second)

	select {}

	// libp2p1 := network.NewLibp2pTransport(nil, nil, nil, nil)
	// libp2p2 := network.NewLibp2pTransport(nil, nil, nil, nil)
	// go libp2p1.Start()
	// time.Sleep(2 * time.Second)
	// go libp2p2.Start()

	// time.Sleep(4 * time.Second)

}

func gen_keys() {
	err := crypto.GenerateKeyPairsJSON(2, "peers.json")
	if err != nil {
		log.Fatalf("Error generating key pairs: %v", err)
	}

	fmt.Println("Key pairs generated successfully and saved to keys.json")
}
