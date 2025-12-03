package dockerclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"time"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/networking/host"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
)

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

	go func() {
		<-ctx.Done()
		_ = dc.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{
			Force: true,
			// TODO: this should be an option. Becccause sometimes we want absolute disposable container.  maybe we just need to disable caching (disable cache volumes resolution)
			RemoveVolumes: false,
		})
	}()

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
