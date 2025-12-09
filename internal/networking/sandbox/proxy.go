package sandbox

import (
	"bufio"
	"context"
	"fmt"
	"net"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/networking/protocol"
	"github.com/0xa1bed0/mkenv/internal/networking/transport"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

type ProxyServer struct {
	rt     *runtime.Runtime
	addr   string
	server *transport.Server
}

func NewProxyServer(rt *runtime.Runtime) *ProxyServer {
	return &ProxyServer{
		addr: fmt.Sprintf("0.0.0.0:%d", hostappconfig.ContainerProxyPort()),
		rt:   rt,
	}
}

func (p *ProxyServer) Run(ctx context.Context) error {
	logs.Infof("[mkenv-agent] proxy listening on %s", p.addr)

	server, err := transport.ServeTCP(p.rt, p.addr, func(servctx context.Context, conn net.Conn) {
		p.handleConn(servctx, conn)
	})
	if err != nil {
		return fmt.Errorf("proxy listen on %s: %w", p.addr, err)
	}
	p.server = server
	defer server.Listener.Close()

	select {
	case <-ctx.Done():
		return nil
	}
}

type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

func (p *ProxyServer) handleConn(ctx context.Context, clientConn net.Conn) {
	logs.Infof("proxy handles connection")
	defer clientConn.Close()

	remote := clientConn.RemoteAddr().String()
	r := bufio.NewReader(clientConn)

	port, err := protocol.ReadProxyHeader(r)
	if err != nil {
		logs.Errorf("[mkenv-agent] proxy: bad header from %s: %v", remote, err)
		return
	}

	targetAddr := fmt.Sprintf("localhost:%d", port)
	backendConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		logs.Errorf("[mkenv-agent] proxy: dial backend %s for %s: %v", targetAddr, remote, err)
		clientConn.Close()
		return
	}
	defer backendConn.Close()

	logs.Infof("[mkenv-agent] proxy: %s -> %s (start)", remote, targetAddr)

	client := &bufferedConn{
		Conn: clientConn,
		r:    r,
	}

	protocol.PumpBidirectional(client, backendConn)

	logs.Infof("[mkenv-agent] proxy: %s -> %s (done)", remote, targetAddr)
}
