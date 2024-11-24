package p2p

import (
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

	// Give the server a second to start
	time.Sleep(1 * time.Second)

	// This slice will hold all the peers for later cleanup
	var peers []*TcpPeer

	// Goroutine to receive peers from the peer channel
	go func() {
		for {
			select {
			case p := <-peerCh:
				log.Printf("peer is %v \n", p)
				peers = append(peers, p)
				log.Printf("peers is %v \n", peers)
			}
		}
	}()

	// Dial 3 times to create 3 peers
	for i := 0; i < 3; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("Failed to dial: %v", err)
		}
		defer conn.Close() // Ensure the connection is closed after the test
	}

	// Wait for peers to be registered
	time.Sleep(1 * time.Second)

	// Broadcast the message
	msg := []byte("hello")
	if err := tr.Broadcast(msg, ""); err != nil {
		log.Printf("Error broadcasting message: %v\n", err)
		panic(err)
	}

	// Use a WaitGroup to wait for all Consume goroutines to finish
	var wg sync.WaitGroup

	// Consume the message on each peer's channel
	for _, p := range peers {
		msgCh := make(chan []byte)
		wg.Add(1)

		// Start a goroutine to consume the message
		go func(p *TcpPeer) {
			defer wg.Done()
			p.Consume(msgCh)

			// Ensure message is consumed correctly
			assert.Equal(t, msg, <-msgCh)
		}(p)
	}

	// Wait for all consume goroutines to complete
	wg.Wait()

	// Close all connections gracefully
	for _, p := range peers {
		err := p.conn.Close()
		if err != nil {
			log.Printf("Error closing connection for peer %v: %v", p.Addr(), err)
		}
	}

	// Clean up the transport
	err := tr.listener.Close() // Close the listener to stop accepting new connections
	if err != nil {
		log.Printf("Error closing transport listener: %v", err)
	}
}

func TestPeerMsgExchange(t *testing.T) {
	// Set up two transporters with different addresses
	addr1 := "localhost:8080"
	addr2 := "localhost:8081"

	peerCh1 := make(chan *TcpPeer)
	peerCh2 := make(chan *TcpPeer)

	tr1 := NewTCPTransport(NetAddr(addr1), peerCh1)
	tr2 := NewTCPTransport(NetAddr(addr2), peerCh2)

	// Start both transporters
	go tr1.Start()
	go tr2.Start()

	// Peers will be added here
	peers := make([]*TcpPeer, 0)

	// Use a goroutine to collect the peers
	go func() {
		for {
			select {
			case p := <-peerCh1:
				log.Printf("tr1 has peer: %v \n", p.Addr())
				peers = append(peers, p)
			case p := <-peerCh2:
				log.Printf("tr2 has peer: %v \n", p.Addr())
				peers = append(peers, p)
			}
		}
	}()

	// Allow time for servers to start
	time.Sleep(2 * time.Second)

	// Establish connections
	err := tr1.Connect(tr2)
	if err != nil {
		t.Fatalf("Failed to connect tr1 to tr2: %v", err)
	}
	err = tr2.Connect(tr1)
	if err != nil {
		t.Fatalf("Failed to connect tr2 to tr1: %v", err)
	}

	// Wait until both peers are connected
	for len(peers) < 2 {
		time.Sleep(100 * time.Millisecond)
	}

	// Ensure that peers are correctly initialized
	peer1 := peers[0]
	peer2 := peers[1]

	log.Printf("Connected peer1: %v", peer1.Addr())
	log.Printf("Connected peer2: %v", peer2.Addr())

	// Send message from tr1 (peer1) to tr2 (peer2)
	err = tr1.SendMsg(NetAddr(peer2.Addr()), []byte("hello from tr1"))
	if err != nil {
		t.Fatalf("Failed to send message from tr1: %v", err)
	}

	// Send message from tr2 (peer2) to tr1 (peer1)
	err = tr2.SendMsg(NetAddr(peer1.Addr()), []byte("hello from tr2"))
	if err != nil {
		t.Fatalf("Failed to send message from tr2: %v", err)
	}

	// Create channels to consume the messages
	msgCh1 := make(chan []byte)
	msgCh2 := make(chan []byte)

	// Start goroutines to consume the messages from each peer
	go peer1.Consume(msgCh1)
	go peer2.Consume(msgCh2)

	// Ensure that the message is received correctly on both sides
	msg1 := <-msgCh1
	msg2 := <-msgCh2

	log.Printf("Received message from peer1: %s", msg1)
	log.Printf("Received message from peer2: %s", msg2)

	// Validate that the correct messages were received
	assert.Equal(t, msg1, []byte("hello from tr2"))
	assert.Equal(t, msg2, []byte("hello from tr1"))
}
