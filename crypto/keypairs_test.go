package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyPairGeneration(t *testing.T) {
	privKey := GeneratePrivateKey()
	assert.NotNil(t, privKey, "Private key should not be nil")

	pubKey := privKey.PublicKey()
	assert.NotNil(t, pubKey.key, "Public key should not be nil")

	// Convert public key to string and back
	pubKeyStr := pubKey.String()
	assert.NotEmpty(t, pubKeyStr, "Public key string should not be empty")

	parsedPubKey, err := StringToPublicKey(pubKeyStr)
	assert.NoError(t, err, "Parsing public key from string should not return an error")

	// Ensure the parsed public key matches the original
	assert.Equal(t, pubKey.String(), parsedPubKey.String(), "Reconstructed public key string should match original")
}

func TestPublicKeyConsistency(t *testing.T) {
	privKey := GeneratePrivateKey()
	pubKey := privKey.PublicKey()

	pubKeyStr := pubKey.String()
	pubKeyBytes := pubKey.Bytes()

	// Convert back from bytes
	parsedPubKey, err := StringToPublicKey(pubKeyStr)
	assert.NoError(t, err, "Parsing public key from bytes should not return an error")

	// Check that bytes match
	assert.Equal(t, hex.EncodeToString(pubKeyBytes), hex.EncodeToString(parsedPubKey.Bytes()), "Public key bytes should match after conversion")
}

func TestSigningAndVerification(t *testing.T) {
	privKey := GeneratePrivateKey()
	pubKey := privKey.PublicKey()

	message := []byte("test message")
	sig, err := privKey.Sign(message)
	assert.NoError(t, err, "Signing should not return an error")

	valid := sig.Verify(message, pubKey)
	assert.True(t, valid, "Signature verification should pass")

	// Test with modified message
	invalid := sig.Verify([]byte("wrong message"), pubKey)
	assert.False(t, invalid, "Signature verification should fail for altered message")
}
