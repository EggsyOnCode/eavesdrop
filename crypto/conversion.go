package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

func ConvertToLibp2pPrivateKey(p *PrivateKey) *Libp2pPrivateKey {
	return &Libp2pPrivateKey{key: p.key}
}

func ConvertToLibp2pPublicKey(p PublicKey) (*Libp2pPublicKey, error) {
	x, y := elliptic.UnmarshalCompressed(secp256k1.S256(), p)
	if x == nil || y == nil {
		return nil, errors.New("invalid compressed public key")
	}

	return &Libp2pPublicKey{key: ecdsa.PublicKey{
		Curve: secp256k1.S256(),
		X:     x,
		Y:     y,
	}}, nil
}
