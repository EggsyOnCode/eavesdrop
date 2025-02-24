package utils

import (
	"crypto/sha256"
)

type NetAddr string

// CompareHashes takes a slice of byte slices, computes their SHA256 hashes,
// and returns true if all hashes are identical.
func CompareHashes(dataItems [][]byte) bool {
	if len(dataItems) == 0 {
		return false
	}

	// Compute hash of the first item
	firstHash := sha256.Sum256(dataItems[0])

	// Compare hashes of all items
	for _, data := range dataItems[1:] {
		if sha256.Sum256(data) != firstHash {
			return false
		}
	}

	return true
}
