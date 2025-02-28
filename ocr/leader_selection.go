package ocr

import (
	"crypto/hmac"
	"crypto/sha256"
	"eavesdrop/logger"
	"encoding/binary"

	"github.com/zyedidia/generic/avl"
)

// selectLeader determines the leader for a given epoch using HMAC-SHA256 as the PRF.
func selectLeader(epoch int, secretKey []byte, numOracles int) int {
	if numOracles < 2 {
		panic("Number of oracles must be at least 2")
	}

	// Convert epoch to byte slice
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, uint64(epoch))

	// Compute HMAC-SHA256(epoch, secretKey)
	h := hmac.New(sha256.New, secretKey)
	h.Write(epochBytes)
	hashed := h.Sum(nil)

	// Convert the first 8 bytes of the hash to an integer
	randomInt := binary.BigEndian.Uint64(hashed[:8])

	// Compute leader index using (Fx(e) mod (n-1)) + 1
	leader := (int(randomInt) % (numOracles - 1)) + 1

	return leader
}

// findLeader finds the leader peer in the AVL tree.
func findLeader(epoch int, secretKey []byte, peers *avl.Tree[string, *ProtcolPeer]) *ProtcolPeer {
	numOracles := peers.Size()
	logger := logger.Get().Sugar()
	if numOracles < 2 {
		logger.Fatal("Not enough peers to select a leader")
	}

	leaderIndex := selectLeader(epoch, secretKey, numOracles)

	// Iterate over the AVL tree in sorted order and return the leader
	i := 1
	var leader *ProtcolPeer

	// we will eventually find something , so no thread of
	// infinite loop here
	peers.Each(func(key string, peer *ProtcolPeer) {
		logger.Info("PACEMAKER in peer Map: peer ", peer.ID)
		if i == leaderIndex {
			leader = peer
			return // Stop iteration early
		}
		i++
	})

	return leader
}
