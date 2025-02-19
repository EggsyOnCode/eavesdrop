package crypto

import (
	"encoding/hex"
	"errors"
)

// address is a 20 bytes hexademical string
// obtained from doing a keccak round on the public key and extracting the last 20 bytes
type Address [20]uint8

func (a *Address) ToSlice() []byte {
	buf := make([]byte, 20)
	for i := 0; i < 20; i++ {
		buf[i] = a[i]
	}
	return buf
}

// AddressFromBytes
func AddressFromBytes(b []byte) Address {
	if len(b) != 20 {
		panic("Binary Address must be 20 bytes long")
	}
	var v [20]uint8
	for i := 0; i < 20; i++ {
		v[i] = b[i]
	}

	return Address(v)
}

// hash is implementing the String interface meaning all its outputs will now be typecasted to  hex string
func (a Address) String() string {
	return "0x" + hex.EncodeToString(a.ToSlice()) 
}


// UnmarshalText implements the encoding.TextUnmarshaler interface for TOML unmarshalling.
func (a *Address) UnmarshalText(text []byte) error {
	hexStr := string(text)

	// Remove '0x' prefix if present
	if len(hexStr) >= 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}

	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return err
	}
	if len(decoded) != 20 {
		return errors.New("invalid address length")
	}
	copy(a[:], decoded)
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface for TOML marshalling.
func (a Address) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}
