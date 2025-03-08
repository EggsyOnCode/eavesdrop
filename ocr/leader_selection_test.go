package ocr

import (
	"testing"

	"github.com/test-go/testify/assert"
	g "github.com/zyedidia/generic"
	"github.com/zyedidia/generic/avl"
)

func TestFindLeaderConsistency(t *testing.T) {
	secretKey := []byte("shared_secret")
	epoch := 42 // Fixed epoch for consistency

	// First order of insertion
	peers1 := avl.New[string, *ProtcolPeer](g.Greater[string])
	peers1.Put("peer2", &ProtcolPeer{ServerID: "peer2"})
	peers1.Put("peer3", &ProtcolPeer{ServerID: "peer3"})
	peers1.Put("peer1", &ProtcolPeer{ServerID: "peer1"})

	// Second order of insertion
	peers2 := avl.New[string, *ProtcolPeer](g.Greater[string])
	peers2.Put("peer3", &ProtcolPeer{ServerID: "peer3"})
	peers2.Put("peer1", &ProtcolPeer{ServerID: "peer1"})
	peers2.Put("peer2", &ProtcolPeer{ServerID: "peer2"})

	// Third order of insertion
	peers3 := avl.New[string, *ProtcolPeer](g.Greater[string])
	peers3.Put("peer1", &ProtcolPeer{ServerID: "peer1"})
	peers3.Put("peer2", &ProtcolPeer{ServerID: "peer2"})
	peers3.Put("peer3", &ProtcolPeer{ServerID: "peer3"})

	// Find leaders for each case
	leader1 := findLeader("peer1", epoch, secretKey, peers1)
	leader2 := findLeader("peer2", epoch, secretKey, peers2)
	leader3 := findLeader("peer3", epoch, secretKey, peers3)

	// Verify leader is not nil
	assert.NotNil(t, leader1, "Leader should not be nil")
	assert.NotNil(t, leader2, "Leader should not be nil")
	assert.NotNil(t, leader3, "Leader should not be nil")

	// Verify all cases return the same leader
	assert.Equal(t, leader1.ServerID, leader2.ServerID, "Leaders should be the same despite insertion order")
	assert.Equal(t, leader1.ServerID, leader3.ServerID, "Leaders should be the same despite insertion order")
}
