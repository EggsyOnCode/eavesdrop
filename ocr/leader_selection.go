package ocr

import (
	"crypto/hmac"
	"crypto/sha256"
	"eavesdrop/logger"
	"encoding/hex"
	"fmt"

	"github.com/zyedidia/generic/avl"
)

// findLeader selects a leader deterministically using hashing instead of indexing.
func findLeader(self string, epoch int, secretKey []byte, peers *avl.Tree[string, *ProtcolPeer]) *ProtcolPeer {
	numOracles := peers.Size()
	logger := logger.Get().Sugar()
	logger.Infof("epoch and secretKey and numOracles: %d, %s, %d", epoch, secretKey, numOracles)

	if numOracles < 1 {
		logger.Errorf("Not enough peers to select a leader")
		return nil
	}

	var selectedPeer *ProtcolPeer
	var minHash string

	// Compare all peer hashes
	peers.Each(func(_ string, peer *ProtcolPeer) {
		peerHash := hashPeer(epoch, secretKey, peer.ServerID)

		// Select the peer with the smallest hash
		if selectedPeer == nil || peerHash < minHash {
			selectedPeer = peer
			minHash = peerHash
		}
	})

	// Also compute and compare self's hash
	selfHash := hashPeer(epoch, secretKey, self)
	if selfHash < minHash {
		logger.Infof("Self has the smallest hash, becoming leader: %s", self)
		return &ProtcolPeer{ServerID: self}
	}

	return selectedPeer
}


// hashPeer computes an HMAC-SHA256 hash for a peer based on epoch, secretKey, and peer ID.
func hashPeer(epoch int, secretKey []byte, peerID string) string {
	h := hmac.New(sha256.New, secretKey)
	h.Write([]byte(peerID)) // Include peer ID
	h.Write([]byte(fmt.Sprintf("%d", epoch)))

	// Convert hash to hex string
	return hex.EncodeToString(h.Sum(nil))
}
