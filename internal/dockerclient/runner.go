package dockerclient

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/docker/docker/api/types/container"
)

const (
	dockerMaxNameLen = 255
	shortLen         = 6       // length of the hash-like suffix
	tailMarker       = "tail-" // visible indicator that we trimmed the left side
)

// RunContainer emulates:
//
//	docker run --rm -it -v ...binds... IMAGE
//
// - uses image's default CMD/ENTRYPOINT (tmux, zsh, etc.)
// - attaches with a real TTY (so tmux + keybindings work)
// - removes container on exit
func (dc *DockerClient) RunContainer(ctx context.Context, projectName, containerID string, claimPort func() error, term *runtime.TerminalGuard) error {
	// Attach BEFORE start (like docker run)
	attach, err := dc.client.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Logs:   false,
	})
	if err != nil {
		return err
	}
	defer attach.Close()

	err = claimPort()
	if err != nil {
		logs.Errorf("can't claim port. error: %v\nlet docker engine claim...", err)
	}

	// Start container
	err = dc.client.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return err
	}

	restoreLogs := logs.Mute()
	defer restoreLogs()

	// --- TTY handling via Guard ---
	if err := term.EnterRawAndWatch(func(width, height uint) {
		_ = dc.client.ContainerResize(ctx, containerID, container.ResizeOptions{
			Height: height,
			Width:  width,
		})
	}); err != nil {
		return err
	}
	defer term.Restore()
	// ------------------------------

	// Let Ctrl+C go *into* tmux/zsh; only treat SIGTERM as "kill from outside".
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGTERM)
	go func() {
		<-stopCh
		_ = dc.client.ContainerKill(context.Background(), containerID, "SIGTERM")
	}()

	// stdin → container
	go func() {
		_, _ = io.Copy(attach.Conn, os.Stdin)
	}()

	// container → stdout (TTY=true: merged)
	go func() {
		_, _ = io.Copy(os.Stdout, attach.Reader)
	}()

	if err := dc.startSandboxDaemon(ctx, projectName, containerID); err != nil {
		// if we can't run container daemon - thhe whole mkenv container is useless
		// TODO: think if the sentence above is true. maybe we should just warn user - not everyone exposes test servers. someone just compiles their binaries and thats it (like mkenv developers working on mkenv.
		return err
	}

	// Wait for container exit
	statusCh, errCh := dc.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case <-ctx.Done():
		attach.Close()
		if err := dc.KillContainer(containerID); err != nil {
			logs.Errorf("can't kill container: error: %v", err)
			return err
		}
	case err := <-errCh:
		attach.Close()
		if err != nil {
			return fmt.Errorf("container wait: %w", err)
		}
		return nil
	case st := <-statusCh:
		attach.Close()
		logs.Debugf("Container exited normally: %v", st)
		return nil
	}

	return nil
}

func (dc *DockerClient) KillContainer(containerID string) error {
	// TODO: close attach
	return dc.client.ContainerKill(context.Background(), containerID, "SIGTERM")
}

func (dc *DockerClient) startSandboxDaemon(ctx context.Context, projectName, containerID string) error {
	// TODO: this does not work everytime for some reason
	execCfg := container.ExecOptions{
		// TODO: move mkenv agent binary path in container as constant
		Cmd:          []string{hostappconfig.AgentBinaryPath(projectName) + "/mkenv", "sandbox", "daemon"},
		AttachStdout: false,
		AttachStderr: false,
		Tty:          false,
	}

	resp, err := dc.client.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return fmt.Errorf("exec create: %w", err)
	}

	// Detach = true: just fire and forget
	if err := dc.client.ContainerExecStart(ctx, resp.ID, container.ExecStartOptions{Detach: true}); err != nil {
		return fmt.Errorf("exec start: %w", err)
	}

	return nil
}
