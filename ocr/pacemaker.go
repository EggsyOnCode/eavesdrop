package ocr

import "eavesdrop/crypto"

type SendingSchme int

const (
	BROADCAST SendingSchme = 0
	LEADER    SendingSchme = 1
	PEER      SendingSchme = 2
)

type Pacemaker struct {
	currEpoch    int               // currEpoch
	highestEpoch int               // highest epoch number sent in NEWEPOCH message
	leader       *crypto.PublicKey // current leader
	msgService   MessagingLayer
}

// if sending Scheme is BROADCAST, send message to all oracles
// if sending Scheme is LEADER, send message to leader (id of hte leader is passed)
// if sending Scheme is PEER, send message to peer (id of the peer is passed)
func (p *Pacemaker) SendMsg(s SendingSchme, msg []byte, id string) {
	switch s {
	case BROADCAST:
		p.msgService.BroadcastMsg(msg)
	case LEADER:
		p.msgService.SendMsg(p.leader.String(), msg)
	case PEER:
		p.msgService.SendMsg(id, msg)
	}
}
