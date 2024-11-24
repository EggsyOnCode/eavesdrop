package network

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// represents a node on the tranport layer
type TcpTransport struct {
	addr     NetAddr
	listener net.Listener
	// mutex lock to protect the peer channel
	mu sync.Mutex
	// listens to incoming conns on socket and sends to peer channel (where it is consumed)
	peerCh  chan *TcpPeer
	peerMap map[NetAddr]*TcpPeer
	msgCh   chan []byte
}

// minimalistic rep of peer wrt to the currently running node
type TcpPeer struct {
	// conn info about the peer
	conn net.Conn
}

func NewTCPPeer(conn net.Conn) *TcpPeer {
	return &TcpPeer{conn: conn}
}

func (tp *TcpPeer) Addr() string {
	return tp.conn.RemoteAddr().String()
}

// sending msg to the peer
func (tp *TcpPeer) SendMsg(msg []byte) error {
	_, err := tp.conn.Write(msg)
	return err
}

// reading from the peer connection
func (tp *TcpPeer) Consume(msgCh chan []byte) {
	buf := make([]byte, 0, 1024) // big buffer
	tmp := make([]byte, 10)      // using small tmo buffer for demonstrating
	for {
		tp.conn.SetReadDeadline(time.Now().Add(5 * time.Second)) // Set a read timeout
		n, err := tp.conn.Read(tmp)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Connection closed by peer")
			} else {
				fmt.Println("Read error:", err)
				break
			}
		}
		fmt.Println("got", n, "bytes.")
		buf = append(buf, tmp[:n]...)
	}

	msgCh <- buf
}

// Dependency injection for peerChannel
func NewTCPTransport(addr NetAddr, peerCh chan *TcpPeer) *TcpTransport {
	return &TcpTransport{
		addr:    addr,
		peerCh:  peerCh,
		mu:      sync.Mutex{},
		peerMap: make(map[NetAddr]*TcpPeer),
	}
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

// accepting incoming connections on the listener and sending them to the peer channel
func (t *TcpTransport) acceptConn() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		log.Printf("Connection accepted: %v", conn.RemoteAddr())

		t.mu.Lock()
		peer := NewTCPPeer(conn)
		t.peerCh <- peer
		t.peerMap[NetAddr(peer.Addr())] = peer
		log.Printf("Peer added: %v", peer.Addr())
		t.mu.Unlock()
	}
}

func (t *TcpTransport) Addr() NetAddr {
	return t.addr
}

// dialing a peer
// TODO: test this func 
func (t *TcpTransport) Connect(tr Transport) error {
	// just Dial a connection and accept loop would automatically
	// add the peer to the peer channel

	// Resolve the remote address to connect to
	remoteAddr, err := net.ResolveTCPAddr("tcp", string(tr.Addr()))
	if err != nil {
		return err
	}

	// Resolve the local address to bind to (using port 8080)
	log.Printf("remote %v and local %v \n", tr.Addr(), t.Addr())
	localAddr, err := net.ResolveTCPAddr("tcp", string(t.addr))
	if err != nil {
		return err
	}

	// Dial the connection, binding to the local address
	_, er := net.DialTCP("tcp", localAddr, remoteAddr)
	if er != nil {
		return er
	}

	return nil
}

// adding peer to the peer channel
func (t *TcpTransport) AddPeer(tr Transport) error {
	// TODO: currently no validation...
	return t.Connect(tr)
}

func (t *TcpTransport) listenToPeer(peer *TcpPeer) {
	defer func() {
		// Close the connection when done
		peer.conn.Close()
	}()

	for {
		buf := make([]byte, 1024) // Allocate buffer
		n, err := peer.conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				fmt.Printf("Connection closed by peer: %s\n", peer.Addr())
			} else {
				fmt.Printf("Error reading from peer %s: %v\n", peer.Addr(), err)
			}
			break // Exit loop on error
		}

		// Send the received message to the message channel
		t.msgCh <- buf[:n]
	}
}

// Consuming msgs from peers
func (t *TcpTransport) Consume() <-chan []byte {
	return t.msgCh
}

// Sending msg to a peer
func (t *TcpTransport) SendMsg(addr NetAddr, msg []byte) error {
	peer, ok := t.peerMap[addr]
	if !ok {
		return fmt.Errorf("Peer not found: %s", addr)
	}

	// Log the peer and the message to be sent
	log.Printf("Sending message to peer %s: %s", peer.Addr(), msg)

	_, err := peer.conn.Write(msg)
	if err != nil {
		log.Printf("Error writing message to peer: %v", err)
	}

	log.Printf("msg sent to peer %s: %s", peer.Addr(), msg)

	return err
}

// Broadcasting msg to all peers
func (t *TcpTransport) Broadcast(msg []byte, exclude NetAddr) error {
	for addr, peer := range t.peerMap {
		if addr == exclude {
			continue
		}

		_, err := peer.conn.Write(msg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *TcpTransport) IsPeer(tr Transport) bool {
	_, ok := t.peerMap[tr.Addr()]
	return ok
}

func (t *TcpTransport) GetPeer(addr NetAddr) *TcpPeer {
	return t.peerMap[addr]
}

func (t *TcpTransport) DeletePeer(addr NetAddr) {
	delete(t.peerMap, addr)
}

// TODO: what if the peer is no longer available, we'll still be broadcasting / consuming from it
// make revisions in respective methods
