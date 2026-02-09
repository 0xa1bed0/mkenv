package dockercontainer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/bricks/systems"
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/dockerclient"
	"github.com/0xa1bed0/mkenv/internal/guardrails"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/networking/host"
	"github.com/0xa1bed0/mkenv/internal/networking/protocol"
	"github.com/0xa1bed0/mkenv/internal/networking/shared"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

type OrchestratorExitSignal struct {
	Err error
}

type ContainerOrchestrator struct {
	rt                *runtime.Runtime
	dockerClient      *dockerclient.DockerClient
	controlAPI        *host.ControlListener
	reverseProxy      *host.ReverseProxyServer
	forwarderRegistry *host.ForwarderRegistry
	exitCh            chan OrchestratorExitSignal

	binds []string

	once sync.Once
}

func NewContainerOrchestrator(rt *runtime.Runtime, binds []string, dockerClient *dockerclient.DockerClient, exitCh chan OrchestratorExitSignal) (*ContainerOrchestrator, error) {
	controlAPI, err := host.StartControlPlane(rt)
	if err != nil {
		return nil, err
	}

	// Load policy for reverse proxy configuration
	policy, err := guardrails.LoadPolicy()
	if err != nil {
		return nil, fmt.Errorf("load policy: %w", err)
	}

	// Start reverse proxy server on random port
	reverseProxy, err := host.StartReverseProxyServer(rt, policy)
	if err != nil {
		return nil, fmt.Errorf("start reverse proxy: %w", err)
	}

	forwarderRegistry := host.NewForwarderRegistry(rt)

	return &ContainerOrchestrator{
		rt:                rt,
		dockerClient:      dockerClient,
		controlAPI:        controlAPI,
		reverseProxy:      reverseProxy,
		forwarderRegistry: forwarderRegistry,
		exitCh:            exitCh,
		binds:             binds,
	}, nil
}

func (co *ContainerOrchestrator) Start() error {
	co.rt.GoNamed("ContainerOrchestrator;startEnv", func() {
		co.startEnv()
	})

	select {
	case exitSignal := <-co.exitCh:
		if exitSignal.Err != nil {
			return exitSignal.Err
		}
		// exited normally - shutdown mkenv
		co.rt.CancelCtx()
		return nil
	case <-co.rt.Ctx().Done():
		logs.Infof("Program exited")
		return nil
	}
}

func (co *ContainerOrchestrator) startEnv() {
	co.once.Do(func() {
		co.controlAPI.ServerProtocol.Handle(co.onPortSnapshot())
		co.controlAPI.ServerProtocol.Handle(co.onExpose())
		co.controlAPI.ServerProtocol.Handle(co.onGetBlockedPorts())
		co.controlAPI.ServerProtocol.Handle(co.onInstallRequest())
		co.controlAPI.ServerProtocol.Handle(co.onLog())
		co.controlAPI.ServerProtocol.Handle(co.onFetchLogs())
	})

	containerCtx, cancelContainer := context.WithCancel(co.rt.Ctx())

	co.rt.Container().SetStopContainer(cancelContainer)

	// Build environment variables including reverse proxy address
	envs := co.getEnvVars()

	containerID, containerPortRessservation, err := co.dockerClient.CreateContainer(containerCtx, co.rt.Project(), co.rt.Container().ImageTag(), envs, co.binds)
	if err != nil {
		co.exitCh <- OrchestratorExitSignal{Err: err}
		return
	}

	co.rt.Container().SetContainerID(containerID)
	co.rt.Container().SetPort(containerPortRessservation.Port)

	errChan := make(chan error, 1)
	co.rt.GoNamed("RunContainer", func() {
		err := co.dockerClient.RunContainer(containerCtx, co.rt.Project().Name(), co.rt.Project().Path(), containerID, containerPortRessservation.Claim, co.rt.Term())
		errChan <- err
	})

	select {
	case err := <-errChan:
		exitSignal := OrchestratorExitSignal{}
		if err != nil {
			exitSignal.Err = err
			logs.Debugf("Container exited with error: %v", err)
		} else {
			logs.Debugf("Container exited")
		}
		co.exitCh <- exitSignal

	case <-containerCtx.Done():
		logs.Infof("container killed from outside")
	}
}

// getEnvVars builds the complete set of environment variables for the container,
// including control API and reverse proxy addresses
func (co *ContainerOrchestrator) getEnvVars() []string {
	envs := make([]string, len(co.controlAPI.Env))
	copy(envs, co.controlAPI.Env)

	// Add reverse proxy address for container to dial back to host
	reverseProxyPort := co.reverseProxy.Port()
	reverseProxyEnv := fmt.Sprintf("MKENV_REVERSE_PROXY=host.docker.internal:%d", reverseProxyPort)
	envs = append(envs, reverseProxyEnv)

	// Pass host timezone so container timestamps match the host
	if tz := hostTimezone(); tz != "" {
		envs = append(envs, "TZ="+tz)
	}

	logs.Debugf("Container env vars: %v", envs)
	return envs
}

// hostTimezone returns the IANA timezone name of the host (e.g. "Europe/Kyiv").
// Works on both macOS and Linux.
func hostTimezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	// macOS: /etc/localtime -> /var/db/timezone/zoneinfo/<tz>
	// Linux: /etc/localtime -> /usr/share/zoneinfo/<tz>
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if idx := strings.Index(target, "zoneinfo/"); idx != -1 {
			return target[idx+len("zoneinfo/"):]
		}
	}
	// Linux (Debian/Ubuntu): /etc/timezone contains the IANA name directly
	if data, err := os.ReadFile("/etc/timezone"); err == nil {
		if tz := strings.TrimSpace(string(data)); tz != "" {
			return tz
		}
	}
	// Fallback to Go's detected location
	if name := time.Now().Location().String(); name != "Local" {
		return name
	}
	return ""
}

func (co *ContainerOrchestrator) onPortSnapshot() (string, protocol.ControlCommandHandler) {
	return "mkenv.sandbox.snapshot", func(ctx context.Context, req protocol.ControlSignalEnvelope) (any, error) {
		var snapshot shared.Snapshot

		err := protocol.UnpackControlSignalEnvelope(req, &snapshot)
		if err != nil {
			return nil, err
		}

		responses := map[int]string{} // port => ok|error

		for port := range snapshot.Listeners {
			err := co.forwarderRegistry.Add(port)
			if err != nil {
				responses[port] = fmt.Sprintf("error while binding port %d: %v", port, err)
				continue
			}
			responses[port] = "ok"
		}

		// Cleanup old listeners
		existingForwarders := co.forwarderRegistry.List()
		for _, port := range existingForwarders {
			if _, ok := snapshot.Listeners[port]; !ok {
				co.forwarderRegistry.Remove(port)
			}
		}

		return &shared.OnSnapshotResponse{Response: responses}, nil
	}
}

func (co *ContainerOrchestrator) onExpose() (string, protocol.ControlCommandHandler) {
	return "mkenv.sandbox.expose", func(ctx context.Context, req protocol.ControlSignalEnvelope) (any, error) {
		var request shared.Expose
		err := protocol.UnpackControlSignalEnvelope(req, &request)
		if err != nil {
			return nil, err
		}

		responses := map[int]string{}

		err = co.forwarderRegistry.Add(request.Listener.Port)
		if err != nil {
			responses[request.Listener.Port] = fmt.Sprintf("error while binding port %d: %v", request.Listener.Port, err)
		} else {
			responses[request.Listener.Port] = "ok"
		}

		return &shared.OnSnapshotResponse{Response: responses}, nil
	}
}

func (co *ContainerOrchestrator) onGetBlockedPorts() (string, protocol.ControlCommandHandler) {
	return "mkenv.sandbox.list-blocked-ports", func(ctx context.Context, req protocol.ControlSignalEnvelope) (any, error) {
		ports, err := host.GetHostBusyPorts(co.rt)
		if err != nil {
			logs.Debugf("error in get host busy ports: %v", err)
			return nil, err
		}

		out := []int{}
		reverseProxyPort := co.reverseProxy.Port()

		for _, port := range ports {
			// Skip ports already forwarded by us
			if co.forwarderRegistry.Has(port) {
				continue
			}

			// CRITICAL: Don't tell container to prebind our reverse proxy port!
			// Otherwise infinite loop: container can't dial reverse proxy because it's blocked
			if port == reverseProxyPort {
				continue
			}

			out = append(out, port)
		}

		logs.Debugf("found blocked ports: %v", out)

		return &shared.BlockedPorts{Ports: out}, nil
	}
}

func (co *ContainerOrchestrator) onInstallRequest() (string, protocol.ControlCommandHandler) {
	return "mkenv.sandbox.install", func(ctx context.Context, req protocol.ControlSignalEnvelope) (any, error) {
		var request shared.Install
		err := protocol.UnpackControlSignalEnvelope(req, &request)
		if err != nil {
			return nil, err
		}

		// TODO: read system from container label
		debian, err := systems.NewDebian(map[string]string{})
		if err != nil {
			return nil, err
		}

		pkgManager := debian.PackageManager()
		if pkgManager == nil {
			// TODO: assume apt manager?
			return nil, errors.New("can't determine system's package manager")
		}

		var response strings.Builder
		cmds := pkgManager.Install([]bricksengine.PackageSpec{{Name: request.PkgName}})
		for _, cmd := range cmds {
			resp, err := co.dockerClient.ExecAsRoot(ctx, co.rt.Container().ContainerID(), cmd.Argv)
			if err != nil {
				return nil, err
			}
			response.WriteString("running: " + strings.Join(cmd.Argv, " ") + "\n\n" + resp + "\n\n")
		}

		return &shared.OnInstallResponse{Logs: response.String()}, nil
	}
}

func (co *ContainerOrchestrator) onLog() (string, protocol.ControlCommandHandler) {
	return "mkenv.sandbox.log", func(ctx context.Context, req protocol.ControlSignalEnvelope) (any, error) {
		var entry shared.LogEntry
		protocol.UnpackControlSignalEnvelope(req, &entry)
		// Write directly to log file (TimestampWriter adds timestamp)
		if w := co.rt.LogWriter(); w != nil {
			line := entry.Line
			if !strings.HasSuffix(line, "\n") {
				line += "\n"
			}
			w.Write([]byte(line))
		}
		return nil, nil
	}
}

func (co *ContainerOrchestrator) onFetchLogs() (string, protocol.ControlCommandHandler) {
	return "mkenv.sandbox.fetch-logs", func(ctx context.Context, req protocol.ControlSignalEnvelope) (any, error) {
		var fetchReq shared.FetchLogsRequest
		protocol.UnpackControlSignalEnvelope(req, &fetchReq)

		logPath := hostappconfig.RunLogPath(co.rt.Project().Name(), co.rt.RunID())
		f, err := os.Open(logPath)
		if err != nil {
			return &shared.FetchLogsResponse{Lines: []string{}, TotalLines: 0}, nil
		}
		defer f.Close()

		var allLines []string
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}

		totalLines := len(allLines)

		// Apply offset and limit
		start := fetchReq.Offset
		if start < 0 {
			start = 0
		}
		if start > totalLines {
			start = totalLines
		}

		end := totalLines
		if fetchReq.Limit > 0 && start+fetchReq.Limit < totalLines {
			end = start + fetchReq.Limit
		}

		return &shared.FetchLogsResponse{
			Lines:      allLines[start:end],
			TotalLines: totalLines,
		}, nil
	}
}
