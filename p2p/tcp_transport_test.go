package p2p

import (
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPeerConnection(t *testing.T) {
	addr := "localhost:8080"
	peerCh := make(chan *TcpPeer)
	tr := NewTCPTransport(NetAddr(addr), peerCh)
	go tr.Start()

	time.Sleep(1 * time.Second)

	// go func() {
	// 	for {
	// 		p := <-peerCh
	// 		log.Printf("peer is %v \n", p)
	// 	}
	// }()
	net.Dial("tcp", addr)
	peer := <-peerCh

	assert.Equal(t, peer, tr.GetPeer(NetAddr(peer.Addr())))
}

func TestPeerCommunication(t *testing.T) {
	// Create an in-memory connection for testing
	serverConn, clientConn := net.Pipe()

	// Create a TcpPeer for the "server" side
	serverPeer := NewTCPPeer(serverConn)

	// Message channel for server to consume messages
	msgCh := make(chan []byte)

	// Start the server's Consume method in a goroutine
	go serverPeer.Consume(msgCh)

	// Create a TcpPeer for the "client" side
	clientPeer := NewTCPPeer(clientConn)

	// Message to send
	msg := []byte("hello")

	// Client sends a message to the server
	err := clientPeer.SendMsg(msg)
	assert.NoError(t, err)

	// Server receives the message
	receivedMsg := <-msgCh
	assert.Equal(t, msg, receivedMsg)

	// Close the connections
	clientConn.Close()
	serverConn.Close()
}

func TestPeerMsgBroadcast(t *testing.T) {
	addr := "localhost:3500"
	peerCh := make(chan *TcpPeer)
	tr := NewTCPTransport(NetAddr(addr), peerCh)
	go tr.Start()

	// Allow the server time to start
	time.Sleep(1 * time.Second)

	// Slice to hold peers (use mutex to synchronize access)
	var peers []*TcpPeer
	var peersLock sync.Mutex

	// Goroutine to collect peers
	go func() {
		for {
			select {
			case p := <-peerCh:
				log.Printf("New peer connected: %v", p.Addr())
				peersLock.Lock()
				peers = append(peers, p)
				peersLock.Unlock()
			}
		}
	}()

	// Dial 3 peers to connect to the transport
	var conns []net.Conn
	for i := 0; i < 3; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("Failed to dial peer %d: %v", i+1, err)
		}
		defer conn.Close() // Ensure connections are closed after the test
		conns = append(conns, conn)
	}

	// Allow time for peers to be registered
	time.Sleep(1 * time.Second)

	// Broadcast the message
	msg := []byte("hello")
	if err := tr.Broadcast(msg, ""); err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	// Use a WaitGroup to ensure all peers receive the message
	var wg sync.WaitGroup

	peersLock.Lock()
	for i, p := range peers {
		wg.Add(1)
		go func(peer *TcpPeer, conn net.Conn) {
			defer wg.Done()

			// Read the message directly from the connection
			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err != nil {
				t.Errorf("Error reading from peer %v: %v", peer.Addr(), err)
				return
			}

			received := buf[:n]
			log.Printf("Peer %v received message: %s", peer.Addr(), string(received))

			// Validate that the message matches the broadcast
			assert.Equal(t, msg, received, "Peer did not receive the expected message")
		}(p, conns[i])
	}
	peersLock.Unlock()

	// Wait for all peers to finish
	wg.Wait()

	// Clean up transport and connections
	if err := tr.listener.Close(); err != nil {
		log.Printf("Error closing transport listener: %v", err)
	}

	peersLock.Lock()
	for _, p := range peers {
		if err := p.conn.Close(); err != nil {
			log.Printf("Error closing connection for peer %v: %v", p.Addr(), err)
		}
	}
	peersLock.Unlock()
}

func TestPeerMsgExchange(t *testing.T) {
	// Set up two transporters with different addresses
	addr1 := "localhost:8080"

	peerCh1 := make(chan *TcpPeer)

	tr1 := NewTCPTransport(NetAddr(addr1), peerCh1)

	// Start both transporters
	go tr1.Start()

	// Peers will be added here
	peers := make([]*TcpPeer, 0)

	// Use a goroutine to collect the peers
	go func() {
		for {
			select {
			case p := <-peerCh1:
				log.Printf("tr1 has peer: %v \n", p.Addr())
				peers = append(peers, p)
			}
		}
	}()

	// Allow time for servers to start
	time.Sleep(1 * time.Second)

	// conn referes to peer -> server conn created upon dialing
	// peer.conn refers to server -> peer conn created upon accepting the connection during
	// listening
	conn, err := net.Dial("tcp", addr1)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Wait for the peer to be added
	time.Sleep(1 * time.Second)

	peer := peers[0]

	// Message to send
	msg := []byte("hello")
	tr1.SendMsg(NetAddr(peer.Addr()), msg)

	// Read the message from the connection
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		if err == io.EOF {
			log.Println("Connection closed by peer")
		} else {
			t.Fatalf("Error reading from connection: %v", err)
		}
	}

	// Truncate buffer to the number of bytes read
	received := buf[:n]

	// Log the received data
	log.Printf("Received %d bytes: %s", n, string(received))

	// Assert the received message matches the sent message
	assert.Equal(t, msg, received)
}
