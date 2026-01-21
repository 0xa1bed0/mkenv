package dockerclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/logs"
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
	// Use folder name as hostname for friendly display in shell prompts
	hostname := sanitizeHostname(filepath.Base(project.Path()))

	cfg := &container.Config{
		Image:    imageTag,
		Hostname: hostname,
		Env:      envs,

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

	for _, bind := range binds {
		logs.Debugf("binding %s", bind)
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
		ExtraHosts: []string{
			"host.docker.internal:host-gateway",
		},
	}

	volumes, err := dc.resolveCacheVolumes(ctx, imageTag, project)
	if err != nil {
		return
	}

	// Resolve cache file store (single volume for all cached files)
	cacheFileStore, err := dc.resolveCacheFileStore(ctx, imageTag, project)
	if err != nil {
		return
	}

	// Track mounted paths to avoid duplicates
	mountedPaths := make(map[string]bool)

	for _, vol := range volumes {
		if mountedPaths[vol.MountPath] {
			continue
		}
		mountedPaths[vol.MountPath] = true
		hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: vol.Name,
			Target: vol.MountPath,
		})
	}

	if cacheFileStore != nil && !mountedPaths[cacheFileStore.MountPath] {
		hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: cacheFileStore.Name,
			Target: cacheFileStore.MountPath,
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

// sanitizeHostname ensures the hostname is valid (RFC 1123):
// - lowercase letters, digits, and hyphens only
// - must start with a letter
// - max 63 characters
func sanitizeHostname(name string) string {
	name = strings.ToLower(name)

	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == '-' || r == '_' || r == '.' || r == ' ' {
			b.WriteRune('-')
		}
	}

	hostname := b.String()

	// Remove leading/trailing hyphens
	hostname = strings.Trim(hostname, "-")

	// Must start with a letter
	if len(hostname) > 0 && (hostname[0] >= '0' && hostname[0] <= '9') {
		hostname = "dev-" + hostname
	}

	// Max 63 characters
	if len(hostname) > 63 {
		hostname = hostname[:63]
	}

	// Fallback if empty
	if hostname == "" {
		hostname = "mkenv"
	}

	return hostname
}
