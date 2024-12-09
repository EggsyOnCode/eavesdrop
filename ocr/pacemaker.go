package ocr

import (
	"eavesdrop/crypto"
	"eavesdrop/rpc"
	"time"

	"github.com/romana/rlog"
)

type SendingSchme int

const (
	BROADCAST SendingSchme = 0
	LEADER    SendingSchme = 1
	PEER      SendingSchme = 2

	ResendTimems time.Duration = time.Second * 3
)

type Pacemaker struct {
	currEpoch     uint64            // currEpoch
	highestEpoch  int               // highest epoch number sent in NEWEPOCH message
	leader        *crypto.PublicKey // current leader
	msgService    MessagingLayer
	TimerResend   *Timer
	TimerProgress *Timer
	quitCh        chan struct{}
	ocrCtx        *OCRCtx
}

func NewPaceMaker() *Pacemaker {
	return &Pacemaker{
		currEpoch:     1,
		highestEpoch:  0,
		leader:        nil,
		TimerResend:   NewTimer(time.Duration(ResendTimems)),
		TimerProgress: NewTimer(10),
		quitCh:        make(chan struct{}),
	}
}

func (p *Pacemaker) SetOCRContext(ctx *OCRCtx) {
	p.ocrCtx = ctx
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

func (p *Pacemaker) AttachMsgLayer(msgService MessagingLayer) {
	p.msgService = msgService
}

func (p *Pacemaker) Start() {
	// senidng NEWEPOCH message after every ∆resend time
	go func() {
		for {
			select {
			case <-p.TimerResend.Subscribe():
				// Broadcast NEWEPOCH message
				payload := &rpc.NewEpochMesage{
					EpochID: p.currEpoch,
				}
				msg, err := rpc.NewMessageBuilder(
					*p.msgService.GetCodec(),
				).SetHeaders(rpc.MessageNewEpoch).SetTopic(rpc.Pacemaker).SetData(payload).Bytes()

				if err != nil {
					panic(err)
				}

				p.msgService.BroadcastMsg(msg)
				rlog.Println("NEWEPOCH message sent")

			case <-p.quitCh: // Add a quit channel for graceful shutdown
				rlog.Println("Pacemaker stopped")
				return
			}
		}
	}()

}

func (p *Pacemaker) ProcessMessage(msg *rpc.DecodedMsg) error {
	switch msg.Data.(type) {
	case *rpc.NewEpochMesage:
		newEMsg := msg.Data.(*rpc.NewEpochMesage)

		rlog.Printf("new epoch msg received.. %+v from %s\n", newEMsg, msg.FromId)
		rlog.Printf("connected node count is : %v\n", p.ocrCtx.PeerCount)
	default:
		rlog.Errorf("RPC Handler: unkown rpc msg header")
	}

	return nil
}
