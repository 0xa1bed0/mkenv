package host

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/0xa1bed0/mkenv/internal/guardrails"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/networking/protocol"
	"github.com/0xa1bed0/mkenv/internal/networking/transport"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

// bufferedConn wraps a net.Conn with a bufio.Reader to handle already-read header bytes
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

// ReverseProxyServer listens on a random host port and proxies container requests
// to host services. This enables containers to access host services (e.g., postgres)
// via localhost from inside the container.
type ReverseProxyServer struct {
	addr   string
	policy guardrails.Policy
	srv    *transport.Server
	once   sync.Once
}

// StartReverseProxyServer creates and starts a reverse proxy server on a random port.
// The container will dial this port when it wants to access host services.
func StartReverseProxyServer(rt *runtime.Runtime, policy guardrails.Policy) (*ReverseProxyServer, error) {
	rps := &ReverseProxyServer{
		policy: policy,
	}

	server, err := transport.ServeTCP(rt, "127.0.0.1:0", func(ctx context.Context, conn net.Conn) {
		rps.handleConn(conn)
	})
	if err != nil {
		return nil, fmt.Errorf("reverse proxy serve: %w", err)
	}

	addr := server.Listener.Addr().String()
	logs.Debugf("reverse proxy server will listen on %s", addr)

	rps.addr = addr
	rps.srv = server

	rt.OnShutdown(func(ctx context.Context) {
		logs.Debugf("Shutdown reverse proxy server...")
		rps.Stop()
	})

	logs.Debugf("reverse proxy server listening on %s", addr)
	return rps, nil
}

// Stop shuts down the reverse proxy server
func (rps *ReverseProxyServer) Stop() {
	rps.once.Do(func() {
		if rps.srv != nil {
			rps.srv.Cancel()
		}
	})
}

// Port extracts the port number from the listen address
func (rps *ReverseProxyServer) Port() int {
	_, portStr, err := net.SplitHostPort(rps.addr)
	if err != nil {
		return 0
	}
	port, _ := strconv.Atoi(portStr)
	return port
}

// handleConn processes a single reverse proxy connection from the container
func (rps *ReverseProxyServer) handleConn(clientConn net.Conn) {
	defer clientConn.Close()

	remote := clientConn.RemoteAddr().String()
	r := bufio.NewReader(clientConn)

	// Read the proxy header: "PORT 5432\n"
	port, err := protocol.ReadProxyHeader(r)
	if err != nil {
		logs.Errorf("reverse proxy: bad header from %s: %v", remote, err)
		return
	}

	// CRITICAL: Check policy - this enforces hardcoded denials + custom policy
	if !rps.policy.AllowReverseProxy(port) {
		logs.Warnf("reverse proxy: port %d denied by policy (from %s)", port, remote)
		return
	}

	// Dial the host service
	targetAddr := fmt.Sprintf("localhost:%d", port)
	backendConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		logs.Errorf("reverse proxy: can't dial %s for %s: %v", targetAddr, remote, err)
		return
	}
	defer backendConn.Close()

	logs.InfofSilent("reverse proxy: container -> host:%d (start)", port)

	// Use bufferedConn to handle already-read header bytes
	client := &bufferedConn{
		Conn: clientConn,
		r:    r,
	}

	protocol.PumpBidirectional(client, backendConn)

	logs.InfofSilent("reverse proxy: container -> host:%d (done)", port)
}
