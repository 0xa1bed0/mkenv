package sandbox

import (
	"bufio"
	"context"
	"fmt"
	"io"
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
	server *transport.TCPServer
}

func NewProxyServer(rt *runtime.Runtime) *ProxyServer {
	return &ProxyServer{
		addr: fmt.Sprintf("0.0.0.0:%d", hostappconfig.ContainerProxyPort()),
		rt:   rt,
	}
}

func (p *ProxyServer) Run(ctx context.Context) error {
	logs.Infof("[mkenv-agent] proxy listening on %s", p.addr)

	server, err := transport.ServeTCP(p.rt, p.addr, func(conn net.Conn) {
		p.handleConn(ctx, conn)
	})
	if err != nil {
		return fmt.Errorf("proxy listen on %s: %w", p.addr, err)
	}
	p.server = server
	defer server.Listener.Close()

	select {
	case <-ctx.Done():
		return nil
	case <-server.Done:
		return fmt.Errorf("proxy listener on %s stopped unexpectedly", p.addr)
	}
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

	targetAddr := fmt.Sprintf("127.0.0.1:%d", port)
	backendConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		logs.Errorf("[mkenv-agent] proxy: dial backend %s for %s: %v", targetAddr, remote, err)
		return
	}
	defer backendConn.Close()

	logs.Infof("[mkenv-agent] proxy: %s -> %s (start)", remote, targetAddr)

	clientRW := struct {
		io.Reader
		io.Writer
	}{Reader: r, Writer: clientConn}

	protocol.PumpBidirectional(clientRW, backendConn)

	logs.Infof("[mkenv-agent] proxy: %s -> %s (done)", remote, targetAddr)
}
