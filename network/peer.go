package network

import (
	"eavesdrop/utils"
)

// minimalistic rep of peer wrt to the currently running node
type Peer struct {
	ID   string
	Addr utils.NetAddr
}

func NewPeer(id string, addr string) *Peer {
	return &Peer{
		ID:   id,
		Addr: utils.NetAddr(addr),
	}
}

// func (tp *Peer) Addr() string {
// 	// TODO: danger null error
// 	if tp.writeSock == nil {
// 		return tp.readSock.RemoteAddr().String()
// 	} else {
// 		return tp.writeSock.RemoteAddr().String()
// 	}
// }

// func (tp *Peer) SetReadSock(conn net.Conn) {
// 	tp.readSock = conn
// }

// func (tp *Peer) SetWriteSock(conn net.Conn) {
// 	tp.writeSock = conn
// }

// func (tp *Peer) WriteSock() net.Conn {
// 	return tp.writeSock
// }

// func (tp *Peer) ReadSock() net.Conn {
// 	return tp.readSock
// }

// // sending msg to the peer
// func (tp *Peer) SendMsg(msg []byte) error {
// 	_, err := tp.writeSock.Write(msg)
// 	return err
// }

// // reading from the peer connection
// func (tp *Peer) Consume(msgCh chan []byte) {
// 	buf := make([]byte, 0, 1024) // big buffer
// 	tmp := make([]byte, 10)      // using small tmo buffer for demonstrating
// 	for {
// 		tp.readSock.SetReadDeadline(time.Now().Add(5 * time.Second)) // Set a read timeout
// 		n, err := tp.readSock.Read(tmp)
// 		if err != nil {
// 			if err == io.EOF {
// 				rlog.Info("Connection closed by peer")
// 			} else {
// 				rlog.Errorf("Read error:", err)
// 				break
// 			}
// 		}
// 		fmt.Println("got", n, "bytes.")
// 		buf = append(buf, tmp[:n]...)
// 	}

// 	msgCh <- buf
// }
