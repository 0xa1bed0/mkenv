package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/spf13/cobra"
)

func NewDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Runs the mkenv sandbox daemon responsible for managing isolated development containers, port forwarding, and runtime control.",
		Long: `The sandbox daemon is a long-running background process that powers mkenv’s
isolated development environments. It manages container lifecycle events,
synchronizes environment state with the host, and provides a control channel
used by the CLI and agents.

Key responsibilities:
  • Monitoring and managing sandboxed containers
  • Port discovery and dynamic host <-> container port forwarding
  • Coordinating agents running inside containers
  • Persisting environment metadata and reacting to changes
  • Handling graceful restarts and crash recovery

The daemon is typically started automatically by the mkenv CLI when an operation
requires an active sandbox, but it can also be run manually for debugging or
advanced setups (e.g., systemd supervision).

It runs indefinitely until stopped and is safe to restart at any time.`,
		RunE: daemonCmdRunE,
	}

	return cmd
}

func daemonCmdRunE(cmd *cobra.Command, args []string) error {
	rt := runtime.FromContextOrPanic(cmd.Context())

	// Best-effort single-instance guard.
	if err := ensureSingleInstance(); err != nil {
		if errors.Is(err, errAlreadyRunning) {
			logs.Infof("ports daemon already running, exiting")
			return nil
		}
		return err
	}

	portsOrchestrator := newPortsOrchestrator(rt)

	portsOrchestrator.StartProxy()
	portsOrchestrator.StartPrebindLoop()
	portsOrchestrator.StartSnapshotReporter()

	rt.Wait()
	logs.Infof("daemon exiting")
	return nil
}

var errAlreadyRunning = errors.New("ports daemon already running")

func ensureSingleInstance() error {
	lockPath, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	lockPath = lockPath + "/.mkenv-daemon-agent.pid"

	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("create lock dir: %w", err)
	}

	pid := os.Getpid()
	pidStr := []byte(strconv.Itoa(pid) + "\n")

	for {
		// Try to create the file atomically.
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			// We own the lock.
			if _, werr := f.Write(pidStr); werr != nil {
				f.Close()
				_ = os.Remove(lockPath)
				return fmt.Errorf("write pid to lock: %w", werr)
			}
			_ = f.Close()
			return nil
		}

		if !os.IsExist(err) {
			return fmt.Errorf("open lock file: %w", err)
		}

		// File exists: check if the recorded PID is still alive.
		data, err := os.ReadFile(lockPath)
		if err != nil {
			// Can't read -> assume stale, try to delete and loop.
			_ = os.Remove(lockPath)
			continue
		}

		existingPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			// Garbage in lock -> nuke and retry.
			_ = os.Remove(lockPath)
			continue
		}

		// TODO: maybe sometimes we can get PID collissions, maybe we should check the process cmd as well
		// Check /proc/<pid>; if it doesn't exist, process is dead -> reclaim.
		if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(existingPID))); err != nil {
			if os.IsNotExist(err) {
				// Stale lock: remove and retry.
				_ = os.Remove(lockPath)
				continue
			}
			// Some other error reading /proc -> bail.
			return fmt.Errorf("stat /proc/%d: %w", existingPID, err)
		}

		// Process is alive: treat as "already running".
		return errAlreadyRunning
	}
}
