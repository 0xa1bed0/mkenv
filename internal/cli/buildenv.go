package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/0xa1bed0/mkenv/internal/cache"
	"github.com/0xa1bed0/mkenv/internal/dockerclient"
	"github.com/0xa1bed0/mkenv/internal/dockerfile"
)

type DockerImageBuildOrchestrator struct {
	cacheManager cache.CacheManager
	planner      dockerfile.Planner
	dockerClient dockerclient.DockerClient
}

func NewDockerImageBuildOrchestrator(dockerClient dockerclient.DockerClient, cacheManager cache.CacheManager, planner dockerfile.Planner) *DockerImageBuildOrchestrator {
	return &DockerImageBuildOrchestrator{
		cacheManager: cacheManager,
		planner:      planner,
		dockerClient: dockerClient,
	}
}

func (orc *DockerImageBuildOrchestrator) ResolveImageTag(ctx context.Context, projectPath string, userPrefs *dockerfile.UserPreferences, forceBuild bool) (string, error) {
	imgID, err := orc.cacheManager.ResolveImage(ctx, projectPath, userPrefs, orc.imageExists, orc.buildDockerfile, orc.buildImageSync, forceBuild)
	if err != nil {
		return "", err
	}

	return string(imgID), nil
}

func (orc *DockerImageBuildOrchestrator) imageExists(ctx context.Context, imgID cache.ImageID) bool {
	// TODO: fix casting
	return orc.dockerClient.ImageExists(ctx, string(imgID))
}

func (orc *DockerImageBuildOrchestrator) buildDockerfile(ctx context.Context) (dockerfile.Dockerfile, error) {
	responseChan := orc.planner.Plan(ctx)
	promptRequestChan := orc.planner.UserPromptsChan()
	promptResponsesChan := orc.planner.UserPromptsResponsesChan()

	for {
		select {
		case prompt, ok := <-promptRequestChan:
			if !ok {
				// planner closed prompt channel; wait for result
				continue
			}
			switch req := prompt.(type) {

			case *dockerfile.Warning:
				fmt.Println("warning:", req.Msg)

			// System / Entrypoint choices come through as requests keyed by your planner
			case *dockerfile.UserInputRequest[dockerfile.BrickID]:
				choice := req.Default
				if choice == "" {
					// no default → pick the first option deterministically
					keys := make([]dockerfile.BrickID, 0, len(req.Options))
					for k := range req.Options {
						keys = append(keys, k)
					}
					// prefer "none" if present (for entrypoint), else the lexicographically smallest key
					if idx := slices.Index(keys, dockerfile.BrickID("none")); idx >= 0 {
						choice = dockerfile.BrickID("none")
					} else {
						slices.SortFunc(keys, func(a, b dockerfile.BrickID) int {
							if a < b {
								return -1
							}
							if a > b {
								return 1
							}
							return 0
						})
						if len(keys) > 0 {
							choice = keys[0]
						}
					}
				}
				promptResponsesChan <- dockerfile.UserInputResponse[any]{Key: req.Key, Reponse: choice}

			default:
				fmt.Printf("info: unhandled prompt %T\n", prompt)
			}

		case res := <-responseChan:
			if res.Err != nil {
				fmt.Println("error:", res.Err)
				return nil, res.Err
			}
			df := res.BuildPlan.GenerateDockerfile()

			return df, nil

		case <-ctx.Done():
			fmt.Println("timed out:", ctx.Err())
			return nil, errors.New("dockerfile generation timeout")
		}
	}
}

func (orc *DockerImageBuildOrchestrator) buildImageSync(ctx context.Context, dockerFile dockerfile.Dockerfile, tag string) (cache.ImageID, error) {
	// TODO: make "mkenv:" in docker tag configurable
	tag, err := orc.dockerClient.BuildImage(ctx, dockerFile.String(), "mkenv:"+tag)
	if err != nil {
		return "", err
	}

	return cache.ImageID(tag), nil
}
