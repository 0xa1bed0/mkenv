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

func (dib *DockerImageResolver) ResolveImageID(ctx context.Context, project *runtime.Project, forceRebuild bool) (ImageID, error) {
	for {
		logs.Debugf("try to resolve image for project: %s", project.Path())
		idByRunConfig, found, runConfigCacheKey := dib.imageCache.GetByProject(ctx, project)
		logs.Debugf("project cache lookup: key=%s, found=%v, imageID=%s", runConfigCacheKey, found, idByRunConfig)
		// we need to set new imaghe to the cache, so we preserve cache key and override result
		// TODO: maybe we don't need to read db in this case - just get cache key and that's it
		if forceRebuild {
			logs.Debugf("forceRebuild=true, ignoring cached image")
			idByRunConfig = ""
			found = false
		}
		if found {
			exists := dib.dockerClient.ImageExists(ctx, string(idByRunConfig))
			logs.Debugf("checking if image exists in Docker: imageRef=%s, exists=%v", idByRunConfig, exists)
			if exists {
				logs.Debugf("cache hit: returning existing image %s", idByRunConfig)
				return idByRunConfig, nil
			}
			logs.Debugf("image not found in Docker, deleting stale cache entry: key=%s", runConfigCacheKey)
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
		logs.Debugf("dockerfile cache lookup: key=%s, found=%v, imageID=%s", dockerfileCacheKey, found, idByDockerfile)
		if forceRebuild {
			logs.Debugf("forceRebuild=true, ignoring dockerfile cache")
			idByRunConfig = ""
			found = false
		}

		if found && !idByDockerfile.IsBuilding() {
			exists := dib.dockerClient.ImageExists(ctx, string(idByDockerfile))
			logs.Debugf("checking if dockerfile-cached image exists: imageRef=%s, exists=%v", idByDockerfile, exists)
			if exists {
				logs.Debugf("dockerfile cache hit: linking project cache and returning image %s", idByDockerfile)
				dib.imageCache.set(ctx, runConfigCacheKey, idByDockerfile)
				return idByDockerfile, nil
			}

			logs.Debugf("dockerfile-cached image not in Docker, deleting stale entry: key=%s", dockerfileCacheKey)
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
		logs.Debugf("claimed building tag: %s", buildingTag)

		imageTag := composeImageTagForProject(project, runConfigCacheKey, dockerfileCacheKey)
		logs.Debugf("building image with tag: %s", imageTag)
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
