package dockerclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/0xa1bed0/mkenv/internal/project"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/moby/term"
)

const (
	dockerMaxNameLen = 255
	shortLen         = 6        // length of the hash-like suffix
	tailMarker       = "tail-"  // visible indicator that we trimmed the left side
)

type cacheVolume struct {
	Name      string
	MountPath string
}

type DockerContainerRunner interface {
	RunContainer(ctx context.Context, proj *project.Project, binds []string) (int64, error)
}

// RunContainer emulates:
//
//	docker run --rm -it -v ...binds... IMAGE
//
// - uses image's default CMD/ENTRYPOINT (tmux, zsh, etc.)
// - attaches with a real TTY (so tmux + keybindings work)
// - removes container on exit
func (dc *dockerClient) RunContainer(ctx context.Context, proj *project.Project, binds []string) (int64, error) {
	controlChan := make(chan int)
	return dc.runContainerRoutine(ctx, proj, binds, controlChan)
	// TODO: this would restart container in the same terminal session - we want to have ability to have control on container from inside container (use unix sockets and rebuild/restart)
	// for {
	// 	time.Sleep(30 * time.Second)
	// 	controlChan<-1
	// 	go dc.runContainerRoutine(ctx, runOpts, controlChan)
	// }
	// return 0, nil
}

// ContainerName: "<project>-<short>", trimming from the LEFT if needed and
// prefixing with "tail-" to show it was trimmed.
func resolveContainerName(project string) string {
	short := shortHash(project +
		"|" + time.Now().UTC().Format(time.RFC3339Nano) +
		"|" + procTag(),
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

func shortHash(s string, n int) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:n]
}

// tiny process tag without extra deps
func procTag() string {
	pid := os.Getpid()
	return hex.EncodeToString([]byte{
		byte(pid >> 24), byte(pid >> 16), byte(pid >> 8), byte(pid),
	})
}

// TODO: make ability to cleanup the caches
func (dc *dockerClient) resolveCacheVolumes(ctx context.Context, proj *project.Project) ([]*cacheVolume, error) {
	out := []*cacheVolume{}

	imageInspect, err := dc.client.ImageInspect(ctx, proj.ImageID)
	if err != nil {
		return nil, err
	}
	requiredVolumes := map[string]bool{}
	if cacheVolumesString, ok := imageInspect.Config.Labels["mkenv_cache_volumes"]; ok {
		for volPath := range strings.SplitSeq(cacheVolumesString, ",") {
			requiredVolumes[volPath] = false
		}
	}

	args := filters.NewArgs()
	args.Add("label", "mkenv_project="+proj.Name)
	existingVolumes, err := dc.client.VolumeList(ctx, volume.ListOptions{
		Filters: args,
	})
	if err != nil {
		return nil, err
	}
	for _, vol := range existingVolumes.Volumes {
		if mountPath, ok := vol.Labels["mkenv_mount_path"]; ok {
			requiredVolumes[mountPath] = true
			out = append(out, &cacheVolume{
				Name:      vol.Name,
				MountPath: mountPath,
			})
		}
	}

	for path, hasVol := range requiredVolumes {
		if hasVol {
			continue
		}

		volName := volumeName(proj.Name, path)
		_, err := dc.client.VolumeCreate(ctx, volume.CreateOptions{
			Name:   volName,
			Driver: "local",
			Labels: map[string]string{
				"mkenv": "1",
				"mkenv_project": proj.Name,
				"mkenv_mount_path": path,
			},
		})
		out = append(out, &cacheVolume{
			Name: volName,
			MountPath: path,
		})
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func volumeName(projName string, path string) string {
	normalizedPath := strings.ReplaceAll(strings.ReplaceAll(path, "/", "_"), ".", "_")
	return "mkenv_cache_volume-"+projName+"-"+normalizedPath
}

func (dc *dockerClient) runContainerRoutine(ctx context.Context, proj *project.Project, binds []string, controlChan chan int) (int64, error) {
	cfg := &container.Config{
		Image:        proj.ImageID,
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

	volumes, err := dc.resolveCacheVolumes(ctx, proj)
	if err != nil {
		return 1, err
	}
	for _, vol := range volumes {
		hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: vol.Name,
			Target: vol.MountPath,
		})
	}

	created, err := dc.client.ContainerCreate(ctx, cfg, hostCfg, nil, nil, resolveContainerName(proj.Name))
	if err != nil {
		return 0, fmt.Errorf("container create: %w", err)
	}
	id := created.ID

	defer func() {
		_ = dc.client.ContainerRemove(context.Background(), id, container.RemoveOptions{
			Force: true,
			// TODO: think of diposable containers but with cache preserves (should be option)
			RemoveVolumes: false,
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

	// IMPORTANT: do initial resize AFTER start so it takes effect immediately
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
	case controlInt := <-controlChan:
		if controlInt == 1 {
			_ = dc.client.ContainerKill(context.Background(), id, "SIGTERM")
		}
	case err := <-errCh:
		if err != nil {
			return 0, fmt.Errorf("container wait: %w", err)
		}
	case st := <-statusCh:
		return st.StatusCode, nil
	}

	return 0, nil
}

func (dc *dockerClient) volumeExists(ctx context.Context, name string) bool {
	_, err := dc.client.VolumeInspect(ctx, name)
	return err == nil
}
