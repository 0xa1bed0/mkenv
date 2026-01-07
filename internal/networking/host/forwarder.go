package host

import (
	"context"
	"fmt"
	"net"
	"sync"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/networking/protocol"
	"github.com/0xa1bed0/mkenv/internal/networking/transport"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

// Forwarder forwards localhost:TargetPort -> localhost:ContainerProxyPort
// the localhost:ContainerProxyPort is the proxy exposed by the container
// The proxy inside the container then proxies to the real in-container server
type Forwarder struct {
	TargetPort         int
	ContainerProxyPort int

	srv  *transport.Server
	once sync.Once
}

func (f *Forwarder) Start(rt *runtime.Runtime) error {
	addr := fmt.Sprintf("0.0.0.0:%d", f.TargetPort)

	server, err := transport.ServeTCP(rt, addr, func(servctx context.Context, conn net.Conn) {
		logs.Debugf("[mkenv host] forwarder accepted client on %d from %s", f.TargetPort, conn.RemoteAddr())
		f.handleConn(conn)
	})
	if err != nil {
		return err
	}

	f.srv = server
	logs.Debugf("[mkenv host] forwarder listening on %s -> container:%d (via proxy at %d)", addr, f.TargetPort, f.ContainerProxyPort)
	return nil
}

func (f *Forwarder) Stop() {
	logs.Debugf("Try stop forwarder server")
	f.once.Do(func() {
		logs.Debugf("Stopping forwarder server")
		if f.srv != nil {
			f.srv.Cancel()
		}
	})
}

func (f *Forwarder) handleConn(clientConn net.Conn) {
	defer clientConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// TODO: make 5 (attempts) configurable
	// TODO: make 0 (delayBetweenAttempts) configurable
	backendConn, err := transport.DialTCP(ctx, fmt.Sprintf("127.0.0.1:%d", f.ContainerProxyPort), 5, 0)
	if err != nil {
		logs.Errorf("[mkenv host] forwarder: DialProxy failed for host port %d: %v", f.TargetPort, err)
		return
	}
	defer backendConn.Close()

	// The proxy chain is always localhost:HostPort -> loclahost(container):ContainerPort (proxy) -> container:HostPort
	// because when something exposes 3000 port in the container (npm run dev) - the same 3000 port must be opened on host
	if err := protocol.WriteProxyHeader(backendConn, f.TargetPort); err != nil {
		logs.Errorf("[mkenv host] forwarder: header write failed on %d: %v", f.TargetPort, err)
		return
	}

	protocol.PumpBidirectional(clientConn, backendConn)
}

type ForwarderRegistry struct {
	runtime    *runtime.Runtime
	mu         sync.Mutex
	forwarders map[int]*Forwarder // key = host port
}

func NewForwarderRegistry(rt *runtime.Runtime) *ForwarderRegistry {
	fr := &ForwarderRegistry{
		forwarders: make(map[int]*Forwarder),
		runtime:    rt,
	}

	rt.OnShutdown(func(ctx context.Context) {
		logs.Debugf("shutdown forwarder")
		fr.StopAll()
	})

	return fr
}

func (r *ForwarderRegistry) Get(targetPort int) (*Forwarder, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.forwarders[targetPort]
	return f, ok
}

func (r *ForwarderRegistry) Add(targetPort int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if targetPort == hostappconfig.ContainerProxyPort() {
		// this port already exposed by the mnkenv
		return nil
	}

	if _, ok := r.forwarders[targetPort]; ok {
		return nil
	}

	// TODO: use r.runtime.OnChildContainerChange
	// TODO: verify runtime is not nil and has container

	f := &Forwarder{
		TargetPort:         targetPort,
		ContainerProxyPort: r.runtime.Container().Port(),
	}

	r.forwarders[targetPort] = f

	return f.Start(r.runtime)
}

func (r *ForwarderRegistry) Has(port int) bool {
	_, ok := r.forwarders[port]
	return ok
}

func (r *ForwarderRegistry) List() []int {
	out := make([]int, len(r.forwarders))
	i := 0
	for port := range r.forwarders {
		out[i] = port
		i++
	}
	return out
}

func (r *ForwarderRegistry) Remove(targetPort int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.forwarders[targetPort]; ok {
		logs.Debugf("Stop forwarding to %d", targetPort)
		f.Stop()
		delete(r.forwarders, targetPort)
	}
}

func (r *ForwarderRegistry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	logs.Debugf("StopAll %v", r.forwarders)
	for port, f := range r.forwarders {
		logs.Debugf("Stop forwarding to %d", port)
		f.Stop()
		delete(r.forwarders, port)
	}
}
