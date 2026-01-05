package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/0xa1bed0/mkenv/internal/networking/protocol"
	"github.com/0xa1bed0/mkenv/internal/networking/shared"
	"github.com/0xa1bed0/mkenv/internal/networking/transport"
)

type ControlClient struct {
	conn *protocol.ControlConn
}

func NewControlClientFromEnv(ctx context.Context) (*ControlClient, error) {
	addr := os.Getenv("MKENV_ADDR")
	if addr == "" {
		return nil, errors.New("MKENV_ADDR missing")
	}
	raw, err := transport.DialTCP(ctx, addr, 200, 50*time.Millisecond)
	if err != nil {
		return nil, err
	}
	cc := protocol.NewControlConn(raw)
	return &ControlClient{conn: cc}, nil
}

func (c *ControlClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *ControlClient) Ping(ctx context.Context) error {
	reqID := protocol.NewID()
	req := protocol.ControlSignalEnvelope{ID: reqID, Type: "ping"}
	_, err := c.conn.Call(ctx, req)
	return err
}

func (c *ControlClient) Snaphost(ctx context.Context, snapshot shared.Snapshot) (*shared.OnSnapshotResponse, error) {
	reqID := protocol.NewID()

	req, err := protocol.PackControlSignalEnvelope(reqID, "mkenv.sandbox.snapshot", snapshot)
	if err != nil {
		return nil, err
	}

	responseEnvelope, err := c.conn.Call(ctx, req)
	if err != nil {
		return nil, err
	}

	var response shared.OnSnapshotResponse
	err = protocol.UnpackControlSignalEnvelope(responseEnvelope, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *ControlClient) Expose(ctx context.Context, port int) error {
	reqID := protocol.NewID()

	exposeRequest := &shared.Expose{Listener: shared.Listener{Port: port}}

	req, err := protocol.PackControlSignalEnvelope(reqID, "mkenv.sandbox.expose", exposeRequest)
	if err != nil {
		return err
	}

	responseEnvelope, err := c.conn.Call(ctx, req)
	if err != nil {
		return err
	}

	var response shared.OnSnapshotResponse
	err = protocol.UnpackControlSignalEnvelope(responseEnvelope, &response)
	if err != nil {
		return err
	}

	if response.Response == nil {
		return fmt.Errorf("port %d is not exposed. unknown error", port)
	}

	if portResp, ok := response.Response[port]; ok {
		if portResp != "ok" {
			return errors.New(portResp)
		}
		return nil
	}

	return fmt.Errorf("port %d is not exposed. unknown error", port)
}

func (c *ControlClient) ListBlockedPorts(ctx context.Context) ([]int, error) {
	reqID := protocol.NewID()

	// TODO: fix nilable generic.
	req, err := protocol.PackControlSignalEnvelope[[]struct{}](reqID, "mkenv.sandbox.list-blocked-ports", nil)
	if err != nil {
		return nil, err
	}

	responseEnvelope, err := c.conn.Call(ctx, req)
	if err != nil {
		return nil, err
	}

	var blockedPortsResponse shared.BlockedPorts
	err = protocol.UnpackControlSignalEnvelope(responseEnvelope, &blockedPortsResponse)
	if err != nil {
		return nil, err
	}

	return blockedPortsResponse.Ports, nil
}

func (c *ControlClient) Install(ctx context.Context, pkgName string) (*shared.OnInstallResponse, error) {
	reqID := protocol.NewID()

	installRequest := &shared.Install{PkgName: pkgName}

	req, err := protocol.PackControlSignalEnvelope(reqID, "mkenv.sandbox.install", installRequest)
	if err != nil {
		return nil, fmt.Errorf("error while packing control signal: %v", err)
	}

	responseEnvelope, err := c.conn.Call(ctx, req)
	if err != nil {
		return nil, err
	}

	var response shared.OnInstallResponse
	err = protocol.UnpackControlSignalEnvelope(responseEnvelope, &response)
	if err != nil {
		return nil, fmt.Errorf("error while unpacking control signal: %v", err)
	}

	return &response, nil
}
