package crypto

// INTENT: To implement the libp2p crypto interface
// use ConvertToLibp2p* methods of this pkg to convert
// PrivateKey and PublicKey to libp2p's PrivKey and PubKey

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	pb "github.com/libp2p/go-libp2p/core/crypto/pb"
)

// Ensure PrivateKey satisfies libp2p's PrivKey interface
var _ libp2pcrypto.PrivKey = (*Libp2pPrivateKey)(nil)

type Libp2pPrivateKey struct {
	key *ecdsa.PrivateKey
}

// Convert private key to raw bytes
func (p *Libp2pPrivateKey) Raw() ([]byte, error) {
	return p.key.D.Bytes(), nil
}

// Returns libp2p key type
func (p *Libp2pPrivateKey) Type() pb.KeyType {
	return pb.KeyType_ECDSA
}

// Sign a message with this private key
func (p *Libp2pPrivateKey) Sign(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, p.key, hash[:])
	if err != nil {
		return nil, err
	}

	rBytes, sBytes := r.Bytes(), s.Bytes()
	sig := append(rBytes, sBytes...)
	return sig, nil
}

// Get the public key corresponding to this private key
func (p *Libp2pPrivateKey) GetPublic() libp2pcrypto.PubKey {
	return &Libp2pPublicKey{p.key.PublicKey}
}

// Compare two private keys
func (p *Libp2pPrivateKey) Equals(other libp2pcrypto.Key) bool {
	o, ok := other.(*Libp2pPrivateKey)
	if !ok {
		return false
	}
	return p.key.D.Cmp(o.key.D) == 0
}

// Ensure PublicKey satisfies libp2p's PubKey interface
var _ libp2pcrypto.PubKey = (*Libp2pPublicKey)(nil)

type Libp2pPublicKey struct {
	key ecdsa.PublicKey
}

// Convert public key to raw bytes
func (p *Libp2pPublicKey) Raw() ([]byte, error) {
	return elliptic.MarshalCompressed(p.key.Curve, p.key.X, p.key.Y), nil
}

// Returns libp2p key type
func (p *Libp2pPublicKey) Type() pb.KeyType {
	return pb.KeyType_ECDSA
}

// Verify a signature
func (p *Libp2pPublicKey) Verify(data []byte, sig []byte) (bool, error) {
	hash := sha256.Sum256(data)

	if len(sig) < 64 {
		return false, errors.New("invalid signature length")
	}

	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])

	return ecdsa.Verify(&p.key, hash[:], r, s), nil
}

// Compare two public keys
func (p *Libp2pPublicKey) Equals(other libp2pcrypto.Key) bool {
	o, ok := other.(*Libp2pPublicKey)
	if !ok {
		return false
	}
	return p.key.X.Cmp(o.key.X) == 0 && p.key.Y.Cmp(o.key.Y) == 0
}
