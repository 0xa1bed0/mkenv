package dockercontainer

import (
	"context"
	"fmt"
	"sync"

	"github.com/0xa1bed0/mkenv/internal/dockerclient"
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

	forwarderRegistry := host.NewForwarderRegistry(rt)

	return &ContainerOrchestrator{
		rt:                rt,
		dockerClient:      dockerClient,
		controlAPI:        controlAPI,
		forwarderRegistry: forwarderRegistry,
		exitCh:            exitCh,
		binds:             binds,
	}, nil
}

func (co *ContainerOrchestrator) Start() error {
	co.rt.Go(func() {
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
	})

	containerCtx, cancelContainer := context.WithCancel(co.rt.Ctx())

	co.rt.Container().SetStopContainer(cancelContainer)

	containerID, containerPortRessservation, err := co.dockerClient.CreateContainer(containerCtx, co.rt.Project(), co.rt.Container().ImageTag(), co.controlAPI.Env, co.binds)
	if err != nil {
		co.exitCh <- OrchestratorExitSignal{Err: err}
		return
	}

	co.rt.Container().SetContainerID(containerID)
	co.rt.Container().SetPort(containerPortRessservation.Port)

	errChan := make(chan error, 1)
	co.rt.Go(func() {
		err := co.dockerClient.RunContainer(containerCtx, co.rt.Project().Name(), containerID, containerPortRessservation.Claim, co.rt.Term())
		errChan <- err
	})

	select {
	case err := <-errChan:
		exitSignal := OrchestratorExitSignal{}
		if err != nil {
			exitSignal.Err = err
		}
		logs.Debugf("Container existed. error: %v", err)
		co.exitCh <- exitSignal

	case <-containerCtx.Done():
		logs.Infof("container killed from outside")
	}
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

		for _, port := range ports {
			if co.forwarderRegistry.Has(port) {
				continue
			}
			out = append(out, port)
		}

		logs.Debugf("found blocked ports: %v", out)

		return &shared.BlockedPorts{Ports: out}, nil
	}
}
