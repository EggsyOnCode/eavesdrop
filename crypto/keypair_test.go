package crypto

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyPairGen(t *testing.T) {
	priv := GeneratePrivateKey()
	pb := priv.PublicKey()
	// address := pb.Address()

	msg := []byte("Hello World")
	sig, err := priv.Sign(msg)
	assert.Nil(t, err)

	verification := sig.Verify(msg, pb)
	assert.True(t, verification)
	log.Println(sig)

}

func TestSignatureFail(t *testing.T) {
	priv := GeneratePrivateKey()
	pb := priv.PublicKey()
	msg := []byte("Hello World")
	sig, err := priv.Sign(msg)
	assert.Nil(t, err)

	verification := sig.Verify([]byte("Hello World2"), pb)
	assert.False(t, verification)
}
