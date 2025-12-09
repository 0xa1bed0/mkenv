package host

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/networking/protocol"
	"github.com/0xa1bed0/mkenv/internal/networking/transport"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

// ControlListener describes how the sandbox/agent should dial back.
type ControlListener struct {
	Network        string                          // "tcp" or "unix"
	Address        string                          // "127.0.0.1:port" or "/path/to.sock"
	Env            []string                        // env vars to inject into container
	ServerProtocol *protocol.ControlServerProtocol // control server
}

// StartControlPlane chooses TCP on darwin and Unix socket elsewhere.
func StartControlPlane(rt *runtime.Runtime) (*ControlListener, error) {
	var (
		ln  net.Listener
		err error
		cl  ControlListener
	)

	// TCP on loopback, random port
	ln, err = transport.ListenTCP("127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("control listen tcp: %w", err)
	}
	addr := ln.Addr().String()

	cl = ControlListener{
		Network: "tcp",
		Address: addr,
		Env: []string{
			"MKENV_RPC=tcp",
			"MKENV_ADDR=host.docker.internal:" + addr[strings.LastIndex(addr, ":")+1:],
		},
	}

	srv := protocol.NewControlServerProtocol(rt, ln) // TODO: make it testable??? - do DI

	cl.ServerProtocol = srv

	rt.GoNamed("Control server", func() {
		err = srv.Serve()
		if err != nil {
			panic(err)
		}
	})

	rt.OnShutdown(func(ctx context.Context) {
		logs.Debugf("Shutdown control server...")
		_ = srv.Close()
	})

	return &cl, nil
}
