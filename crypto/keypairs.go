package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
)

type PrivateKey struct {
	key *ecdsa.PrivateKey
}

// ConvertBase64ToECDSAPrivateKey converts a base64 string to *ecdsa.PrivateKey
func ConvertBase64ToECDSAPrivateKey(base64Key string) (*PrivateKey, error) {
	// Decode Base64 string
	keyBytes, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Base64 key: %w", err)
	}

	// Validate key length (32 bytes for secp256r1)
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32 bytes for secp256r1, got %d", len(keyBytes))
	}

	// Create the private key scalar (d)
	d := new(big.Int).SetBytes(keyBytes)

	// Use secp256r1 curve
	curve := elliptic.P256()

	// Construct the private key
	privateKey := &ecdsa.PrivateKey{
		D: d,
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
		},
	}

	// Derive the public key from the private key
	privateKey.PublicKey.X, privateKey.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())
	if privateKey.PublicKey.X == nil || privateKey.PublicKey.Y == nil {
		return nil, fmt.Errorf("invalid private key: public key coordinates are nil")
	}

	return &PrivateKey{key: privateKey}, nil
}

func NewPrivateKeyUsingReader(r io.Reader) *PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), r)
	if err != nil {
		panic(err)
	}

	return &PrivateKey{key: key}
}

// the SECP256k1 curve is used in generating the private,public key pairs
// the private key returned also has an embedded field for the public key
func GeneratePrivateKey() *PrivateKey {
	return NewPrivateKeyUsingReader(rand.Reader)
}

// msg are signed with PrivateKey
func (p *PrivateKey) Sign(data []byte) (*Signature, error) {

	r, s, err := ecdsa.Sign(rand.Reader, p.key, data)
	if err != nil {
		return nil, err
	}

	return &Signature{R: r, S: s}, nil
}

type PublicKey []byte

func (p *PrivateKey) PublicKey() PublicKey {
	return elliptic.MarshalCompressed(elliptic.P256(), p.key.X, p.key.Y)
}

func StringToPublicKey(hexString string) (PublicKey, error) {
	bytes, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %v", err)
	}
	return PublicKey(bytes), nil
}

// slice of bytes which would be sent over the network

func (p PublicKey) String() string {
	return hex.EncodeToString(p)
}
func (p PublicKey) Address() Address {
	hash := sha256.Sum256(p)

	//the last 20 bytes are the address
	return AddressFromBytes(hash[len(hash)-20:])
}

type Signature struct {
	R, S *big.Int
}

func (sig *Signature) String() string {
	return sig.R.String() + sig.S.String()
}

// msg can be verified with the public key
func (sig *Signature) Verify(data []byte, p PublicKey) bool {
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), p)
	pk := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}
	return ecdsa.Verify(pk, data, sig.R, sig.S)
}
