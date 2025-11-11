package dockerclient

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/api/types/container"
	"github.com/moby/term"
)

type DockerContainerRunner interface {
	RunContainer(ctx context.Context, imageName string, binds []string) (int64, error)
}

// RunContainer emulates:
//   docker run --rm -it -v ...binds... IMAGE
// - uses image's default CMD/ENTRYPOINT (tmux, zsh, etc.)
// - attaches with a real TTY (so tmux + keybindings work)
// - removes container on exit
func (dc *dockerClient) RunContainer(ctx context.Context, imageName string, binds []string) (int64, error) {
	cfg := &container.Config{
		Image:        imageName,
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		// use image's CMD/ENTRYPOINT (tmux/zsh/etc)
	}

	hostCfg := &container.HostConfig{
		Binds: binds,
	}

	created, err := dc.client.ContainerCreate(ctx, cfg, hostCfg, nil, nil, "")
	if err != nil {
		return 0, fmt.Errorf("container create: %w", err)
	}
	id := created.ID

	defer func() {
		_ = dc.client.ContainerRemove(context.Background(), id, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})
	}()

	// Put local terminal in raw mode so tmux/zsh get real key events.
	inFd, isTerm := term.GetFdInfo(os.Stdin)
	var oldState *term.State
	if isTerm {
		oldState, err = term.MakeRaw(inFd)
		if err != nil {
			return 0, fmt.Errorf("make raw: %w", err)
		}
		defer term.RestoreTerminal(inFd, oldState)
	}

	// Attach BEFORE start (like docker run)
	attach, err := dc.client.ContainerAttach(ctx, id, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Logs:   false,
	})
	if err != nil {
		return 0, fmt.Errorf("container attach: %w", err)
	}
	defer attach.Close()

	// Start container
	if err := dc.client.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return 0, fmt.Errorf("container start: %w", err)
	}

	// 🔴 IMPORTANT: do initial resize AFTER start so it takes effect immediately
	if isTerm {
		if ws, err := term.GetWinsize(inFd); err == nil {
			_ = dc.client.ContainerResize(ctx, id, container.ResizeOptions{
				Height: uint(ws.Height),
				Width:  uint(ws.Width),
			})
		}
	}

	// Watch for future resizes (SIGWINCH)
	if isTerm {
		resizeCh := make(chan os.Signal, 1)
		signal.Notify(resizeCh, syscall.SIGWINCH)
		go func() {
			for range resizeCh {
				if ws, err := term.GetWinsize(inFd); err == nil {
					_ = dc.client.ContainerResize(context.Background(), id, container.ResizeOptions{
						Height: uint(ws.Height),
						Width:  uint(ws.Width),
					})
				}
			}
		}()
	}

	// Let Ctrl+C go *into* tmux/zsh; only treat SIGTERM as "kill from outside".
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGTERM)
	go func() {
		<-stopCh
		_ = dc.client.ContainerKill(context.Background(), id, "SIGTERM")
	}()

	// stdin → container
	go func() {
		_, _ = io.Copy(attach.Conn, os.Stdin)
	}()

	// container → stdout (TTY=true: merged)
	go func() {
		_, _ = io.Copy(os.Stdout, attach.Reader)
	}()

	// Wait for container exit
	statusCh, errCh := dc.client.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return 0, fmt.Errorf("container wait: %w", err)
		}
	case st := <-statusCh:
		return st.StatusCode, nil
	}

	return 0, nil
}

