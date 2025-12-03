package host

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
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

	if rt.GOOS() == "darwin" {
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
	} else {
		unixPath := hostappconfig.AgentBinaryPath(rt.Project().Name()) + "/api.sock"
		// Unix domain socket
		if err := os.MkdirAll(filepath.Dir(unixPath), 0o755); err != nil {
			return nil, fmt.Errorf("control mkdir: %w", err)
		}
		ln, err = transport.ListenUnix(unixPath)
		if err != nil {
			return nil, fmt.Errorf("control listen unix: %w", err)
		}
		_ = os.Chmod(unixPath, 0o666)

		cl = ControlListener{
			Network: "unix",
			Address: unixPath,
			Env: []string{
				"MKENV_RPC=unix",
				"MKENV_SOCK=" + unixPath, // path inside container; you can map unixPath -> this
			},
		}
	}

	srv := protocol.NewControlServerProtocol(rt, ln) // TODO: make it testable??? - do DI

	cl.ServerProtocol = srv

	rt.Go(func() {
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
