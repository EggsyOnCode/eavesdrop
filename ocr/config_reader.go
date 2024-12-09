package ocr

import (
	"encoding/json"
	"fmt"
	"os"
)

type PeerInfo struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	ListenAddr string `json:"listenAddr"`
}

// Interface for reading network config like peers.json / Contract of OCR etc
type ConfigReader interface {
	GetPeerInfo() ([]PeerInfo, error)
}

// Implements ConfigReader interface for reading .json file

type JsonConfigReader struct {
	FileName string
}

func NewJsonConfigReader(fileName string) *JsonConfigReader {
	return &JsonConfigReader{
		FileName: fileName,
	}
}

func (j *JsonConfigReader) GetPeerInfo() ([]PeerInfo, error) {
	// Read the JSON file
	data, err := os.ReadFile(j.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read peer file: %w", err)
	}

	var peers []PeerInfo
	if err := json.Unmarshal(data, &peers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal peers: %w", err)
	}

	return peers, nil
}
