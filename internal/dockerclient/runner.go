package dockerclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	sandboxappconfig "github.com/0xa1bed0/mkenv/internal/apps/sandbox/config"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/docker/docker/api/types/container"
)

// setTerminalTitle sets the terminal tab/window title using ANSI escape sequences.
func setTerminalTitle(title string) {
	// OSC 0 sets both window title and icon name
	fmt.Fprintf(os.Stdout, "\033]0;%s\007", title)
}

const (
	dockerMaxNameLen = 255
	shortLen         = 6       // length of the hash-like suffix
	tailMarker       = "tail-" // visible indicator that we trimmed the left side
)

func (dc *DockerClient) AttachToRunning(ctx context.Context, containerID, displayName string, term *runtime.TerminalGuard) error {
	cont, err := dc.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return err
	}

	if !cont.State.Running {
		return errors.New("can't attach to non running container")
	}

	// Set terminal title
	if displayName != "" {
		setTerminalTitle(displayName)
	}

	attachCmd := strings.Split(cont.Config.Labels["mkenv.attachInstruction"], "|MKENVSEP|")

	// Create an exec session that attaches/creates tmux
	// (swap bash/sh/zsh as you prefer)
	execResp, err := dc.client.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          attachCmd,
	})
	if err != nil {
		return fmt.Errorf("exec create: %w", err)
	}

	hijack, err := dc.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return fmt.Errorf("exec attach: %w", err)
	}
	defer hijack.Close()

	restoreLogs := logs.Mute()
	defer restoreLogs()

	// 3) Enter raw + resize EXEC on start + on every window change
	err = term.EnterRawAndWatch(func(width, height uint) {
		_ = dc.client.ContainerExecResize(ctx, execResp.ID, container.ResizeOptions{
			Height: height,
			Width:  width,
		})
	})
	if err != nil {
		return err
	}
	defer term.Restore()

	// Optional but recommended: force one resize immediately (some watchers only fire on change)
	w, h, err := term.Size() // if you have this; otherwise call your terminal size getter
	if err != nil {
		return err
	}
	_ = dc.client.ContainerExecResize(ctx, execResp.ID, container.ResizeOptions{Width: w, Height: h})

	// IMPORTANT: start stdout pump immediately
	// Wrap stdout with filter to preserve terminal content on exit
	filteredOut := runtime.NewAltScreenFilter(os.Stdout)
	outErr := make(chan error, 1)
	go func() {
		_, e := io.Copy(filteredOut, hijack.Reader) // TTY=true => raw stream (no stdcopy)
		filteredOut.Flush()
		outErr <- e
	}()

	// stdin -> container
	inErr := make(chan error, 1)
	go func() {
		_, e := io.Copy(hijack.Conn, os.Stdin)
		inErr <- e
	}()

	// Wait for session end / cancel.
	// (Exec session ends when tmux detach/exit happens; container can keep running.)
	select {
	case <-ctx.Done():
		hijack.Close()
		return nil

	case e := <-outErr:
		hijack.Close()
		// io.Copy often returns nil/EOF when session closes — treat as normal.
		if e != nil && !errors.Is(e, io.EOF) {
			return fmt.Errorf("stdout copy: %w", e)
		}
		return nil

	case e := <-inErr:
		hijack.Close()
		if e != nil && !errors.Is(e, io.EOF) {
			return fmt.Errorf("stdin copy: %w", e)
		}
		return nil
	}
}

// RunContainer emulates:
//
//	docker run --rm -it -v ...binds... IMAGE
//
// - uses image's default CMD/ENTRYPOINT (tmux, zsh, etc.)
// - attaches with a real TTY (so tmux + keybindings work)
// - removes container on exit
func (dc *DockerClient) RunContainer(ctx context.Context, projectName, projectPath, containerID string, claimPort func() error, term *runtime.TerminalGuard) error {
	// Set terminal title to the folder name
	folderName := filepath.Base(projectPath)
	setTerminalTitle(folderName)
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

	err = term.EnterRawAndWatch(func(width, height uint) {
		_ = dc.client.ContainerResize(ctx, containerID, container.ResizeOptions{
			Height: height,
			Width:  width,
		})
	})
	if err != nil {
		return err
	}
	defer term.Restore()

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
	// Wrap stdout with filter to preserve terminal content on exit
	filteredOut := runtime.NewAltScreenFilter(os.Stdout)
	go func() {
		_, _ = io.Copy(filteredOut, attach.Reader)
		filteredOut.Flush()
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
		logs.Debugf("ctx.Done() called")
		if err := dc.KillContainer(containerID); err != nil {
			logs.Errorf("can't kill container: error: %v", err)
			return err
		}
	case err := <-errCh:
		attach.Close()
		logs.Debugf("err chan triggered")
		if err != nil {
			return fmt.Errorf("container wait: %w", err)
		}
		if err := dc.KillContainer(containerID); err != nil {
			logs.Errorf("can't kill container: error: %v", err)
			return err
		}
		return nil
	case st := <-statusCh:
		attach.Close()
		logs.Debugf("Container exited normally: %v", st)
		err := dc.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{RemoveVolumes: false, Force: true})
		if err != nil {
			logs.Errorf("can't remove container %s: %v", containerID, err)
		}
		return nil
	}

	logs.Debugf("wtf?")
	return nil
}

func (dc *DockerClient) KillContainer(containerID string) error {
	// TODO: close attach
	err := dc.client.ContainerKill(context.Background(), containerID, "SIGTERM")
	if err != nil {
		return err
	}
	return dc.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{RemoveVolumes: false, Force: true})
}

func (dc *DockerClient) startSandboxDaemon(ctx context.Context, projectName, containerID string) error {
	execCfg := container.ExecOptions{
		Cmd:          []string{sandboxappconfig.UserLocalBin + "/mkenv", "sandbox", "daemon"},
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

func (dc *DockerClient) ExecAsRoot(ctx context.Context, containerID string, cmd []string) (string, error) {
	execCfg := container.ExecOptions{
		User:         "root",
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	resp, err := dc.client.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return "", fmt.Errorf("exec create: %w", err)
	}

	// Attach to get output streams
	hijack, err := dc.client.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{
		Tty: false,
	})
	if err != nil {
		return "", fmt.Errorf("exec attach: %w", err)
	}
	defer hijack.Close()

	// Start the exec
	err = dc.client.ContainerExecStart(ctx, resp.ID, container.ExecStartOptions{})
	if err != nil {
		return "", fmt.Errorf("exec start: %w", err)
	}

	// Read all output
	output, err := io.ReadAll(hijack.Reader)
	if err != nil {
		return "", fmt.Errorf("read output: %w", err)
	}

	// Check exit code
	inspectResp, err := dc.client.ContainerExecInspect(ctx, resp.ID)
	if err != nil {
		return string(output), fmt.Errorf("exec inspect: %w", err)
	}

	if inspectResp.ExitCode != 0 {
		return string(output), fmt.Errorf("command failed with exit code %d", inspectResp.ExitCode)
	}

	return string(output), nil
}
