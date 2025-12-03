package dockerimage

import (
	"context"
	"time"

	"github.com/0xa1bed0/mkenv/internal/dockerclient"
	"github.com/0xa1bed0/mkenv/internal/dockerfile"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

type DockerImageResolver struct {
	dockerClient *dockerclient.DockerClient
	imageCache   *DockerImageCache
}

func NewDockerImageResolver(dockerClient *dockerclient.DockerClient, imageCache *DockerImageCache) *DockerImageResolver {
	return &DockerImageResolver{
		dockerClient: dockerClient,
		imageCache:   imageCache,
	}
}

var defaultDockerImageResolver *DockerImageResolver

func DefaultDockerImageResolver(ctx context.Context) (*DockerImageResolver, error) {
	if defaultDockerImageResolver == nil {
		dockerClient, err := dockerclient.DefaultDockerClient()
		if err != nil {
			return nil, err
		}
		imageCache := DefaultNaiveDockerImageCache(ctx)
		defaultDockerImageResolver = NewDockerImageResolver(dockerClient, imageCache)
	}

	return defaultDockerImageResolver, nil
}

func (dib *DockerImageResolver) ResolveImageID(ctx context.Context, project *runtime.Project) (ImageID, error) {
	for {
		logs.Debugf("try to resolve image")
		idByRunConfig, found, runConfigCacheKey := dib.imageCache.GetByProject(ctx, project)
		if found {
			if dib.dockerClient.ImageExists(ctx, string(idByRunConfig)) {
				return idByRunConfig, nil
			}
			dib.imageCache.delete(ctx, runConfigCacheKey)
		}

		// TODO: think of dependency injection and unit tests if necessary. We want planner initialize lazy
		dockerfilePlanner := dockerfile.NewPlanner(project)

		logs.Debugf("image cache miss. start dockerfile planning...")
		// TODO: make plan once because user may answer the same questions about entrypoint and system again and again...
		// we can keep their answers (store inside run config) or just PlanOnce
		// also, we can think of GenerateDockerfileOnce - but this require significantly less system resources so maybe don't bother
		plan, err := dockerfilePlanner.Plan(ctx)
		if err != nil {
			return ImageID(""), err
		}

		logs.Debugf("generating dockerfile...")
		df := plan.GenerateDockerfile()
		idByDockerfile, found, dockerfileCacheKey := dib.imageCache.GetByDockerfile(ctx, df)

		if found && !idByDockerfile.IsBuilding() {
			if dib.dockerClient.ImageExists(ctx, string(idByDockerfile)) {
				dib.imageCache.set(ctx, runConfigCacheKey, idByDockerfile)
				return idByDockerfile, nil
			}

			dib.imageCache.delete(ctx, dockerfileCacheKey)
		}

		if found && idByDockerfile.IsBuilding() {
			// TODO: can we log buildlog from other process and don't wait if there is no one?
			logs.Warnf("Found other mkenv process that builds the same image. Waiting it... (Restart mkenv with --build to not wait)")
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(1 * time.Second):
			}
			continue
		}

		logs.Debugf("dockerfile cache miss. Starting building...")

		buildingTag := dib.imageCache.ClaimBuilding(ctx, runConfigCacheKey, dockerfileCacheKey)

		imageTag := composeImageTagForProject(project, runConfigCacheKey, dockerfileCacheKey)
		dockerImageID, err := dib.dockerClient.BuildImage(ctx, df.String(), imageTag)
		if err != nil {
			dib.imageCache.StopBuilding(ctx, runConfigCacheKey, buildingTag)
			dib.imageCache.StopBuilding(ctx, dockerfileCacheKey, buildingTag)
			return "", err
		}

		dib.imageCache.set(ctx, runConfigCacheKey, ImageID(dockerImageID))
		dib.imageCache.set(ctx, dockerfileCacheKey, ImageID(dockerImageID))

		return ImageID(dockerImageID), nil
	}
}
