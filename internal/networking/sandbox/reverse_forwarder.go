package sandbox

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/networking/protocol"
)

// ReverseForwarder listens on a specific port inside the container and forwards
// all connections to the host's reverse proxy server, which then proxies to
// the actual host service on the same port.
//
// Example: Container app calls localhost:5432 -> ReverseForwarder -> Host Reverse Proxy -> Host Postgres
type ReverseForwarder struct {
	port         int
	reverseProxy string // host.docker.internal:12345
	listener     net.Listener
	ctx          context.Context
	cancel       context.CancelFunc
	once         sync.Once
}

// NewReverseForwarder creates a new reverse forwarder for a specific port
func NewReverseForwarder(port int, reverseProxyAddr string) *ReverseForwarder {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReverseForwarder{
		port:         port,
		reverseProxy: reverseProxyAddr,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins listening on the specified port and accepting connections
func (rf *ReverseForwarder) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", rf.port))
	if err != nil {
		return fmt.Errorf("reverse forwarder listen on port %d: %w", rf.port, err)
	}
	rf.listener = ln

	logs.Infof("reverse forwarder listening on 127.0.0.1:%d -> %s", rf.port, rf.reverseProxy)

	go rf.acceptLoop()
	return nil
}

// acceptLoop continuously accepts new connections and spawns goroutines to handle them
func (rf *ReverseForwarder) acceptLoop() {
	for {
		conn, err := rf.listener.Accept()
		if err != nil {
			select {
			case <-rf.ctx.Done():
				// Clean shutdown
				return
			default:
				logs.Errorf("reverse forwarder accept on port %d: %v", rf.port, err)
				return
			}
		}
		go rf.handleConn(conn)
	}
}

// handleConn processes a single connection from a container app
func (rf *ReverseForwarder) handleConn(clientConn net.Conn) {
	defer clientConn.Close()

	// Dial the host's reverse proxy server
	hostConn, err := net.DialTimeout("tcp", rf.reverseProxy, 5*time.Second)
	if err != nil {
		logs.Errorf("reverse forwarder: can't dial %s: %v", rf.reverseProxy, err)
		return
	}
	defer hostConn.Close()

	// Send the port header to tell the host which port we want to access
	if err := protocol.WriteProxyHeader(hostConn, rf.port); err != nil {
		logs.Errorf("reverse forwarder: can't write header for port %d: %v", rf.port, err)
		return
	}

	logs.Debugf("reverse forwarder: proxying container:%d -> host:%d", rf.port, rf.port)

	// Pump data bidirectionally
	protocol.PumpBidirectional(clientConn, hostConn)
}

// Close shuts down the reverse forwarder
func (rf *ReverseForwarder) Close() error {
	var err error
	rf.once.Do(func() {
		rf.cancel()
		if rf.listener != nil {
			err = rf.listener.Close()
		}
	})
	return err
}
