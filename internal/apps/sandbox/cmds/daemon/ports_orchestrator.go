package daemon

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/networking/sandbox"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

type prebinds struct {
	mu               sync.Mutex
	ports            map[int]io.Closer
	reverseProxyAddr string
}

func newPrebinds(reverseProxyAddr string) *prebinds {
	return &prebinds{
		ports:            map[int]io.Closer{},
		reverseProxyAddr: reverseProxyAddr,
	}
}

func (p *prebinds) Has(port int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.ports[port]
	return ok
}

func (p *prebinds) Remove(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if closer, ok := p.ports[port]; ok {
		err := closer.Close()
		if err != nil {
			logs.Errorf("error while closing prebind: %v", err)
		}
		delete(p.ports, port)
	}
}

func (p *prebinds) Add(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.ports[port]; ok {
		return
	}

	// If reverse proxy is configured, create a forwarder instead of blocking
	if p.reverseProxyAddr != "" {
		logs.Infof("creating reverse forwarder for host port %d -> %s", port, p.reverseProxyAddr)

		forwarder := sandbox.NewReverseForwarder(port, p.reverseProxyAddr)
		if err := forwarder.Start(); err != nil {
			logs.Errorf("can't start reverse forwarder on port %d: %v", port, err)
			return
		}

		p.ports[port] = forwarder
		return
	}

	// Fallback: old behavior - just block the port with noop listener
	logs.Infof("prebinding port %d as it is claimed by host (blocking mode)", port)

	closer, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		logs.Errorf("can't prebind port: %v", err)
		return
	}

	p.ports[port] = closer
}

func (p *prebinds) CloseFilter(filter func(port int) bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for port, closer := range p.ports {
		logs.Debugf("test if %d port should be closed", port)
		if filter(port) {
			logs.Infof("closing port %d via closer: %v", port, closer)
			err := closer.Close()
			if err != nil {
				logs.Errorf("error while closing prebind: %v", err)
			}
			delete(p.ports, port)
		}
	}
}

type portsOrchestrator struct {
	prebinds    *prebinds
	rt          *runtime.Runtime
	controlConn *sandbox.ControlClient
}

func newPortsOrchestrator(rt *runtime.Runtime) *portsOrchestrator {
	conn, err := sandbox.NewControlClientFromEnv(rt.Ctx())
	if err != nil {
		panic(fmt.Errorf("dial control server: %w", err))
	}
	rt.OnShutdown(func(context.Context) {
		_ = conn.Close()
	})

	// Get reverse proxy address from environment
	reverseProxyAddr := os.Getenv("MKENV_REVERSE_PROXY")
	if reverseProxyAddr == "" {
		logs.Warnf("MKENV_REVERSE_PROXY not set, reverse proxy disabled (ports will be blocked)")
	} else {
		logs.Infof("Reverse proxy enabled, will forward to %s", reverseProxyAddr)
	}

	return &portsOrchestrator{
		prebinds:    newPrebinds(reverseProxyAddr),
		rt:          rt,
		controlConn: conn,
	}
}

func (po *portsOrchestrator) StartPrebindLoop() {
	ctx := po.rt.Ctx()

	po.rt.GoNamed("PrebindLoop", func() {
		// TODO: make configurable
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				resp, err := po.controlConn.ListBlockedPorts(ctx)
				if err != nil {
					logs.Errorf("error while requesting opened ports from host: %v", err)
					continue
				}

				// this is needed to reiterate on prebinds and cleanup those who does not reported by host (released by host)
				respMap := map[int]bool{}

				for _, port := range resp {
					respMap[port] = true

					po.prebinds.Add(port)
				}

				po.prebinds.CloseFilter(func(port int) bool {
					_, ok := respMap[port]
					return !ok
				})
			}
		}
	})
}

func (po *portsOrchestrator) StartProxy() {
	ctx := po.rt.Ctx()
	po.rt.GoNamed("Proxy", func() {
		proxy := sandbox.NewProxyServer(po.rt)
		if err := proxy.Run(ctx); err != nil {
			logs.Errorf("proxy error: %v", err)
			// If proxy blows up, stop daemon - mkenv is unusable.
			po.rt.CancelCtx()
		}
	})
}

func (po *portsOrchestrator) StartSnapshotReporter() {
	ctx := po.rt.Ctx()

	po.rt.GoNamed("SnaphostReporter", func() {
		// TODO: make configurable
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snap, err := sandbox.CollectSnapshot()
				if err != nil {
					logs.Errorf("collect snapshot error: %v", err)
					continue
				}

				resp, err := po.controlConn.Snaphost(ctx, snap)
				if err != nil {
					logs.Errorf("send snapshot error: %v", err)
					// TODO: what should we do with error?
					continue
				}

				for port, bindResult := range resp.Response {
					if bindResult == "ok" {
						continue
					}

					if po.prebinds.Has(port) {
						continue
					}

					if l, ok := snap.Listeners[port]; ok && l.PID > 1 {
						po.killProcess(l.PID, port, bindResult)
					}
				}

				// if port was in snapshot but host ignores it
				for port := range snap.Listeners {
					if _, ok := resp.Response[port]; !ok {
						if po.prebinds.Has(port) {
							continue
						}

						if l, ok := snap.Listeners[port]; ok && l.PID > 1 {
							po.killProcess(l.PID, port, "unknown error. host ignored this port")
						}
					}
				}
			}
		}
	})
}

func (po *portsOrchestrator) killProcess(pid, port int, reason string) {
	logs.Infof("killing proccess %d because port %d binding failed: %v", pid, port, reason)
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		logs.Errorf("deny: kill pid %d: %v", pid, err)
	}
}
