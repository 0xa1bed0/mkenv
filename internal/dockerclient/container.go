package dockerclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/networking/host"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
)

type MkenvContainerInfo struct {
	ContainerID string
	Name        string
	State       string
	Status      string
	Created     string
	Command     string
	Project     string
}

func (ci *MkenvContainerInfo) OptionLabel() string {
	return fmt.Sprintf("%s - %s (created at: %s)", ci.Project, ci.Name, ci.Created)
}

func (ci *MkenvContainerInfo) OptionID() string {
	return ci.ContainerID
}

func (dc *DockerClient) ListAllContainer(ctx context.Context, runningOnly bool) ([]*MkenvContainerInfo, error) {
	args := filters.NewArgs()
	args.Add("label", "mkenv=true")
	result, err := dc.client.ContainerList(ctx, container.ListOptions{
		Filters: args,
	})
	if err != nil {
		return nil, err
	}

	out := []*MkenvContainerInfo{}

	for _, container := range result {
		if runningOnly && container.State != "running" {
			continue
		}
		created := time.Unix(container.Created, 0).Local().Format(time.RFC1123Z)
		out = append(out, &MkenvContainerInfo{
			ContainerID: container.ID,
			Name:        strings.Join(container.Names, ","),
			State:       container.State,
			Status:      container.Status,
			Created:     created,
			Command:     container.Command,
			Project:     container.Labels["mkenv.project"],
		})
	}

	return out, nil
}

func (dc *DockerClient) ListContainers(ctx context.Context, project *runtime.Project, runningOnly bool) ([]*MkenvContainerInfo, error) {
	args := filters.NewArgs()
	args.Add("label", "mkenv.project="+project.Name())
	result, err := dc.client.ContainerList(ctx, container.ListOptions{
		Filters: args,
	})
	if err != nil {
		return nil, err
	}

	out := []*MkenvContainerInfo{}

	for _, container := range result {
		if runningOnly && container.State != "running" {
			continue
		}
		created := time.Unix(container.Created, 0).Local().Format(time.RFC1123Z)
		out = append(out, &MkenvContainerInfo{
			ContainerID: container.ID,
			Name:        strings.Join(container.Names, ","),
			State:       container.State,
			Status:      container.Status,
			Created:     created,
			Command:     container.Command,
			Project:     project.Name(),
		})
	}

	return out, nil
}

func (dc *DockerClient) CreateContainer(ctx context.Context, project *runtime.Project, imageTag string, envs, binds []string) (containerID string, containerPortReservation *host.PortReservation, err error) {
	cfg := &container.Config{
		Image: imageTag,
		Env:   envs,

		// use image's CMD/ENTRYPOINT (tmux/zsh/etc)

		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Labels: map[string]string{
			"mkenv.project": project.Name(),
		},
	}

	containerPortReservation = host.ReserveFreeTCPPort()
	if containerPortReservation.Err != nil {
		err = containerPortReservation.Err
		return
	}

	hostCfg := &container.HostConfig{
		Binds: binds,
		PortBindings: nat.PortMap{
			nat.Port(strconv.Itoa(hostappconfig.ContainerProxyPort()) + "/tcp"): []nat.PortBinding{
				{
					HostIP:   "127.0.0.1",
					HostPort: strconv.Itoa(containerPortReservation.Port),
				},
			},
		},
	}

	volumes, err := dc.resolveCacheVolumes(ctx, imageTag, project)
	if err != nil {
		return
	}

	for _, vol := range volumes {
		hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: vol.Name,
			Target: vol.MountPath,
		})
	}

	created, err := dc.client.ContainerCreate(ctx, cfg, hostCfg, nil, nil, resolveContainerName(project.Name()))
	if err != nil {
		return
	}
	containerID = created.ID

	return
}

// ContainerName: "<project>-<short>", trimming from the LEFT if needed and
// prefixing with "tail-" to show it was trimmed.
func resolveContainerName(project string) string {
	short := shortHash(project+
		"|"+time.Now().UTC().Format(time.RFC3339Nano)+
		"|"+procTag(),
		shortLen)

	// Ideal: project + "-" + short
	need := len(project) + 1 + len(short)
	if need <= dockerMaxNameLen {
		return project + "-" + short
	}

	// Not enough room. Keep the tail of project and add a visible marker.
	maxProject := dockerMaxNameLen - 1 - len(short) // room for '-' + short
	keep := maxProject - len(tailMarker)
	if keep < 1 {
		keep = 1
	}
	if keep > len(project) {
		keep = len(project)
	}
	trimmedTail := project[len(project)-keep:]

	return tailMarker + trimmedTail + "-" + short
}

// tiny process tag without extra deps
func procTag() string {
	pid := os.Getpid()
	return hex.EncodeToString([]byte{
		byte(pid >> 24), byte(pid >> 16), byte(pid >> 8), byte(pid),
	})
}

func shortHash(s string, n int) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:n]
}
