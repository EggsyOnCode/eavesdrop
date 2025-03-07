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

func (pk *PrivateKey) Key() *ecdsa.PrivateKey {
	return pk.key
}

// ConvertBase64ToECDSAPrivateKey converts a base64 string to *ecdsa.PrivateKey using secp256k1.
func ConvertBase64ToECDSAPrivateKey(base64Key string) (*PrivateKey, error) {
	// Decode Base64 string
	keyBytes, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Base64 key: %w", err)
	}

	// Validate key length (32 bytes for secp256k1)
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32 bytes for secp256k1, got %d", len(keyBytes))
	}

	// Create the private key scalar (d)
	d := new(big.Int).SetBytes(keyBytes)

	// Use secp256k1 curve
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

type PublicKey struct {
	key *ecdsa.PublicKey
}

func (p *PrivateKey) PublicKey() PublicKey {
	return PublicKey{key: &p.key.PublicKey}
}

func (p PublicKey) Bytes() []byte {
	// Convert to compressed format
	x := p.key.X.Bytes()
	prefix := byte(0x02)
	if p.key.Y.Bit(0) == 1 { // Check if Y is odd - more efficient
		prefix = byte(0x03)
	}

	pubKey := make([]byte, 1+len(x))
	pubKey[0] = prefix
	copy(pubKey[1:], x)

	return pubKey
}

// msg are signed with PrivateKey
func (p *PrivateKey) Sign(data []byte) (*Signature, error) {
	r, s, err := ecdsa.Sign(rand.Reader, p.key, data)
	if err != nil {
		return nil, err
	}

	return &Signature{R: r, S: s}, nil
}

func StringToPublicKey(hexString string) (PublicKey, error) {
	bytes, err := hex.DecodeString(hexString)
	if err != nil {
		return PublicKey{}, fmt.Errorf("invalid hex string: %v", err)
	}

	// Unmarshal the compressed public key
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), bytes)
	if x == nil || y == nil {
		return PublicKey{}, fmt.Errorf("invalid compressed public key")
	}

	return PublicKey{key: &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}}, nil
}

// slice of bytes which would be sent over the network

func (p PublicKey) String() string {
	return hex.EncodeToString(p.Bytes())
}
func (p PublicKey) Address() Address {
	hash := sha256.Sum256(p.Bytes())

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
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), p.Bytes())
	pk := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}
	return ecdsa.Verify(pk, data, sig.R, sig.S)
}
