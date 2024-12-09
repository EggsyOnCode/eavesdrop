package crypto

import (
	"encoding/json"
	"fmt"
	"os"
)

type KeyPair struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	ListenAddr string `json:"listenAddr"`
}

func GenerateKeyPairsJSON(x int, outputFile string) error {
	keyPairs := make([]KeyPair, 0, x)

	for i := 0; i < x; i++ {
		// Generate private key
		privKey := GeneratePrivateKey()

		// Encode private key in Base64
		privateKeyhex := fmt.Sprintf("%x", privKey.key.D.Bytes())

		// Encode public key in Base64
		publicKeyBytes := privKey.PublicKey()
		publicKeyhex := publicKeyBytes.String()

		// Append to the list
		keyPairs = append(keyPairs, KeyPair{
			PrivateKey: privateKeyhex,
			PublicKey:  publicKeyhex,
			ListenAddr: "127.0.0.1:4000", // Leave empty to fill later
		})
	}

	// Convert keyPairs to JSON
	jsonData, err := json.MarshalIndent(keyPairs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write JSON to file
	err = os.WriteFile(outputFile, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}
