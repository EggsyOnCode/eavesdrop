package network

import (
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/romana/rlog"
)

// represents a node on the tranport layer
type TcpTransport struct {
	addr     utils.NetAddr
	listener net.Listener
	// mutex lock to protect the peer channel
	mu sync.Mutex

	// passed as dependencies from the Server
	peerCh chan *Peer
	msgCh  chan *rpc.RPCMessage
	// default codec is json, we can set custom using setCodec
	codec rpc.Codec
}

// minimalistic rep of peer wrt to the currently running node
type Peer struct {
	// conn info about the peer
	// writeSock
	writeSock net.Conn
	// readSock
	readSock net.Conn
}

func NewPeer(writeSock net.Conn) *Peer {
	return &Peer{writeSock: writeSock}
}

func (tp *Peer) Addr() string {
	// TODO: danger null error
	if tp.writeSock == nil {
		return tp.readSock.RemoteAddr().String()
	} else {
		return tp.writeSock.RemoteAddr().String()
	}
}

func (tp *Peer) SetReadSock(conn net.Conn) {
	tp.readSock = conn
}

func (tp *Peer) SetWriteSock(conn net.Conn) {
	tp.writeSock = conn
}

func (tp *Peer) WriteSock() net.Conn {
	return tp.writeSock
}

func (tp *Peer) ReadSock() net.Conn {
	return tp.readSock
}

// sending msg to the peer
func (tp *Peer) SendMsg(msg []byte) error {
	_, err := tp.writeSock.Write(msg)
	return err
}

// reading from the peer connection
func (tp *Peer) Consume(msgCh chan []byte) {
	buf := make([]byte, 0, 1024) // big buffer
	tmp := make([]byte, 10)      // using small tmo buffer for demonstrating
	for {
		tp.readSock.SetReadDeadline(time.Now().Add(5 * time.Second)) // Set a read timeout
		n, err := tp.readSock.Read(tmp)
		if err != nil {
			if err == io.EOF {
				rlog.Info("Connection closed by peer")
			} else {
				rlog.Errorf("Read error:", err)
				break
			}
		}
		fmt.Println("got", n, "bytes.")
		buf = append(buf, tmp[:n]...)
	}

	msgCh <- buf
}

// Dependency injection for peerChannel
func NewTCPTransport(addr utils.NetAddr, peerCh chan *Peer, msgCh chan *rpc.RPCMessage) *TcpTransport {
	return &TcpTransport{
		addr:   addr,
		peerCh: peerCh,
		mu:     sync.Mutex{},
		msgCh:  msgCh,
		codec:  rpc.NewJsonCodec(),
	}
}

func (t *TcpTransport) SetCodec(c rpc.Codec) {
	t.codec = c
}

func (t *TcpTransport) Start() {
	// listen on addr, for incoming connections
	listener, err := net.Listen("tcp", string(t.addr))
	if err != nil {
		panic(err)
	}

	t.listener = listener

	go t.acceptConn()
}

func (t *TcpTransport) Stop() error {
	return t.listener.Close()
}

// accepting incoming connections on the listener and sending them to the peer channel
func (t *TcpTransport) acceptConn() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		rlog.Infof("Accepted connection from %s\n", conn.RemoteAddr())

		t.mu.Lock()
		// incoming connection are rep using readSockets...
		p := NewPeer(nil)
		p.SetReadSock(conn)
		t.peerCh <- p
		t.mu.Unlock()
	}
}

func (t *TcpTransport) Addr() utils.NetAddr {
	return t.addr
}

// dialing a peer
func (t *TcpTransport) Connect(addr utils.NetAddr) error {
	// Just dial a connection to the remote peer's address
	conn, err := net.Dial("tcp", string(addr))
	if err != nil {
		return fmt.Errorf("failed to dial remote address: %w", err)
	}

	// Wrap the connection in a Peer
	peer := &Peer{
		writeSock: conn,
	}

	// Safely add the peer to the peerCh
	t.mu.Lock()
	defer t.mu.Unlock()

	// ctx: we don't listen to writeSockets
	// // listening to Peer
	// go t.ListenToPeer(peer)

	// Optionally send to peerCh for other consumers to process
	select {
	case t.peerCh <- peer:
	default:
		// Log if peerCh is full
		log.Printf("Peer channel full; peer %v not sent to channel", peer.Addr())
	}

	log.Printf("Successfully connected to peer: %v", peer.Addr())
	return nil
}

func (t *TcpTransport) ListenToPeer(peer *Peer) {
	defer func() {
		// Close the connection when done
		peer.readSock.Close()
	}()

	log.Printf("Listening to peer %s\n", peer.Addr())

	for {
		buf := make([]byte, 1024) // Allocate buffer
		n, err := peer.readSock.Read(buf)
		if err != nil {
			if err == io.EOF {
				rlog.Errorf("Connection closed by peer: %s\n", peer.Addr())
			} else {
				fmt.Printf("Error reading from peer %s: %v\n", peer.Addr(), err)
			}
			break // Exit loop on error
		}

		// decode here received bytes into an RPC message
		rpcMsg := &rpc.RPCMessage{}
		if err := t.codec.Decode(buf[:n], rpcMsg); err != nil {
			panic("error decoding incoing msg")
		}

		rpcMsg.FromSock = utils.NetAddr(peer.Addr())

		t.msgCh <- rpcMsg
	}
}

// Consuming peers from the peer channel
func (t *TcpTransport) ConsumePeers() <-chan *Peer {
	return t.peerCh
}

// Consuming msgs from peers
func (t *TcpTransport) ConsumeMsgs() <-chan *rpc.RPCMessage {
	return t.msgCh
}

// Sending msg to a peer
func (t *TcpTransport) SendMsg(peer *Peer, msg []byte) error {
	// no need to check existence, is ensured by Server

	// Log the peer and the message to be sent
	log.Printf("Sending message to peer %s: %s", peer.Addr(), msg)

	_, err := peer.writeSock.Write(msg)
	if err != nil {
		log.Printf("Error writing message to peer: %v", err)
	}

	log.Printf("msg sent to peer %s: %s", peer.Addr(), msg)

	return err
}

// Broadcasting msg to all peers
func (t *TcpTransport) Broadcast(msg []byte, peers []*Peer, exclude utils.NetAddr) error {
	for _, peer := range peers {
		if utils.NetAddr(peer.Addr()) == exclude {
			continue
		}

		_, err := peer.writeSock.Write(msg)
		if err != nil {
			return err
		}
	}

	return nil
}

// TODO: what if the peer is no longer available, we'll still be broadcasting / consuming from it
// make revisions in respective methods
