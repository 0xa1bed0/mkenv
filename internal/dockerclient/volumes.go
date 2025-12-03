package dockerclient

import (
	"context"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

type cacheVolume struct {
	Name      string
	MountPath string
}

// TODO: make ability to cleanup the caches
func (dc *DockerClient) resolveCacheVolumes(ctx context.Context, imageTag string, project *runtime.Project) ([]*cacheVolume, error) {
	out := []*cacheVolume{}

	imageInspect, err := dc.client.ImageInspect(ctx, imageTag)
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
	args.Add("label", "mkenv_project="+project.Name())
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

		volName := volumeName(project.Name(), path)
		_, err := dc.client.VolumeCreate(ctx, volume.CreateOptions{
			Name:   volName,
			Driver: "local",
			Labels: map[string]string{
				"mkenv":            "1",
				"mkenv_project":    project.Name(),
				"mkenv_mount_path": path,
			},
		})
		out = append(out, &cacheVolume{
			Name:      volName,
			MountPath: path,
		})
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func (dc *DockerClient) volumeExists(ctx context.Context, name string) bool {
	_, err := dc.client.VolumeInspect(ctx, name)
	return err == nil
}

func volumeName(projName string, path string) string {
	normalizedPath := strings.ReplaceAll(strings.ReplaceAll(path, "/", "_"), ".", "_")
	return "mkenv_cache_volume-" + projName + "-" + normalizedPath
}
