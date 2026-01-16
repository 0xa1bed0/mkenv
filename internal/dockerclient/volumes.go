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
	args.Add("label", "mkenv.project="+project.Name())
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
				"mkenv.project":    project.Name(),
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

// resolveCacheFileStore reads the mkenv_cache_file_store label from the image
// and returns a volume configuration for the cache file store directory.
// Files are stored in this directory and symlinked to their expected paths.
func (dc *DockerClient) resolveCacheFileStore(ctx context.Context, imageTag string, project *runtime.Project) (*cacheVolume, error) {
	imageInspect, err := dc.client.ImageInspect(ctx, imageTag)
	if err != nil {
		return nil, err
	}

	cacheFileStore, ok := imageInspect.Config.Labels["mkenv_cache_file_store"]
	if !ok || cacheFileStore == "" {
		return nil, nil
	}

	// Check for existing volume
	args := filters.NewArgs()
	args.Add("label", "mkenv.project="+project.Name())
	args.Add("label", "mkenv.cache_file_store=true")
	existingVolumes, err := dc.client.VolumeList(ctx, volume.ListOptions{
		Filters: args,
	})
	if err != nil {
		return nil, err
	}

	for _, vol := range existingVolumes.Volumes {
		if mountPath, ok := vol.Labels["mkenv_mount_path"]; ok && mountPath == cacheFileStore {
			return &cacheVolume{
				Name:      vol.Name,
				MountPath: mountPath,
			}, nil
		}
	}

	// Create the volume
	volName := cacheFileStoreVolumeName(project.Name())
	_, err = dc.client.VolumeCreate(ctx, volume.CreateOptions{
		Name:   volName,
		Driver: "local",
		Labels: map[string]string{
			"mkenv":                  "1",
			"mkenv.project":          project.Name(),
			"mkenv.cache_file_store": "true",
			"mkenv_mount_path":       cacheFileStore,
		},
	})
	if err != nil {
		return nil, err
	}

	return &cacheVolume{
		Name:      volName,
		MountPath: cacheFileStore,
	}, nil
}

func cacheFileStoreVolumeName(projName string) string {
	return "mkenv_cache_file_store-" + projName
}
