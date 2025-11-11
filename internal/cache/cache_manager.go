package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/0xa1bed0/mkenv/internal/dockerfile"
)

type (
	CacheKey string
	ImageID  string
)

type Cache struct {
	cacheFilePath string // JSON file
	mu   FSMutex
}

type CacheManager interface {
	ResolveImage(
		ctx context.Context,
		projectPath string,
		userPreferences *dockerfile.UserPreferences,
		imageExists func(context.Context, ImageID) bool,
		buildDockerfile func(ctx context.Context) (dockerFile dockerfile.Dockerfile, err error),
		buildImageSync func(ctx context.Context, dockerFile dockerfile.Dockerfile) (ImageID, error),
	) (ImageID, error)
}

// TODO: make sensible
const (
	buildingStaleAfter = 30 * time.Minute
	buildingPrefix     = "BUILDING:" // full format: BUILDING:<unixTs>:<dfSig>
)

func NewCacheManager(path string) (CacheManager, error) {
	if path == "" {
		return nil, errors.New("cache path required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	c := &Cache{
		cacheFilePath: path,
		mu:   NewFSMutex(path + ".lock"),
	}

	return c, nil
}

func (c *Cache) ResolveImage(
	ctx context.Context,
	projectPath string,
	userPreferences *dockerfile.UserPreferences,

	imageExists func(context.Context, ImageID) bool,
	buildDockerfile func(ctx context.Context) (dockerFile dockerfile.Dockerfile, err error),
	buildImageSync func(ctx context.Context, dockerFile dockerfile.Dockerfile) (ImageID, error),
) (ImageID, error) {
	if imageExists == nil || buildImageSync == nil || buildDockerfile == nil {
		return "", errors.New("helpers imageExists, buildImage, and buildDockerfile is mandatory for image resolving")
	}

	hasValidUserPrefsKey := true
	userPreferenceKey, err := CacheKeyUserPreferences(projectPath, userPreferences)
	if err != nil {
		hasValidUserPrefsKey = false
	}


	for {
		// we are not fully rely on cache.
		// mkenv is a tool for creating environment, the main goal is to <u>create</u> working envrionment,
		// not create it <u>efficient</u>. Also, as a reminder, docker itself has build cache,
		// so even if we repeat build there are huge chances it will be fast.
		// so if we can't work with our own image cache then we anyway want to create workspace for the user.
		// because of that we omit all cache errors and assume them as warnings.
		// TODO: refactor this flag
		readOnlyState := !hasValidUserPrefsKey

		// 40 means "wait 40 times for 50 milliseconds. ~2 seconds"
		// TODO: fix API to be able to provide timeout in seconds
		if !readOnlyState { // if readonly state was set above then skip all locks
			if err := c.mu.Lock(40); err != nil {
				readOnlyState = true
			}
		}

		state, stateLoadErr := c.loadState(readOnlyState)
		if stateLoadErr != nil {
			// if we could lock but could not read the state
			// then let's unlock mutext earlier and work with empty readonly state
			// unlocking earlier is fine because mutext is idempotent
			c.mu.Unlock()
			readOnlyState = true
			state = newReadonlyEmptyCacheState()
		}

		if id, ok := state.getImageIDByUserPreferenceKey(userPreferenceKey); ok {
			if isBuilding(id) {
				// the image is building by another process. lets wait a little bit and try again.
				c.mu.Unlock()
				time.Sleep(150 * time.Millisecond)
				continue
			}
			if imageExists(ctx, id) {
				c.mu.Unlock()
				return id, nil
			}
			_ = state.cleanupUserPreferenceKey(userPreferenceKey)
		}

		// we don't want to keep cache locked while building dockerfile.
		c.mu.Unlock()
		dockerFile, dockerfileBuildErr := buildDockerfile(ctx)
		if dockerfileBuildErr != nil {
			return "", dockerfileBuildErr
		}

		if !readOnlyState {
			if err := c.mu.Lock(40); err != nil {
				readOnlyState = true
			}
		}

		state, stateLoadErr = c.loadState(readOnlyState)
		if stateLoadErr != nil {
			if readOnlyState {
				state = newReadonlyEmptyCacheState()
			} else {
				state = newEmptyCacheState(c.cacheFilePath)
			}
		}

		dockerfileKey := CacheKeyDockerfileLines(dockerFile)

		// TODO: fix CacheKey string casting
		if id, ok := state.getImageIDByDockerfileKey(dockerfileKey); ok {
			_ = state.setUserPreferenceKey(userPreferenceKey, id)
			if isBuilding(id) {
				// the image is building by another process. lets wait a little bit and try again.
				c.mu.Unlock()
				time.Sleep(150 * time.Millisecond)
				continue
			}

			if imageExists(ctx, id) {
				c.mu.Unlock()
				return id, nil
			}

			_ = state.cleanupImageID(userPreferenceKey, dockerfileKey)
		}

		buildingID := newBuildingID(string(dockerfileKey))
		_ = state.setImageID(userPreferenceKey, dockerfileKey, buildingID)
		// we don't want to keep cache locked while building image.
		c.mu.Unlock()

		dockerImageID, dockerImageBuildErr := buildImageSync(ctx, dockerFile)
		if dockerImageBuildErr != nil {
			if e := c.mu.Lock(40); e != nil {
				// we don't care about cache. See comment above
				return "", dockerImageBuildErr
			}

			if s, err := c.loadState(false); err == nil {
				if cur, ok := s.DockerfileKeyToImage[dockerfileKey]; ok && cur == buildingID {
					_ = s.cleanupImageID(userPreferenceKey, dockerfileKey)
				}
			}

			c.mu.Unlock()
			return "", dockerImageBuildErr
		}

		if err := c.mu.Lock(40); err != nil {
			return dockerImageID, nil
		}

		if s, err := c.loadState(false); err == nil {
			// we intentionally override whatever sits there because from security stand point we trust only ourselves
			// and we don't want arbitrary cache state editing process (remember it's just a file in filesystem) to put
			// malicious workspace image
			// we understand that this is not a silver buller and we anyway rely on cache earlier. We will get to that later.
			_ = s.setImageID(userPreferenceKey, dockerfileKey, dockerImageID)
		}

		c.mu.Unlock()
		return dockerImageID, nil
	}
}

func newBuildingID(dfSig string) ImageID {
	now := time.Now().Unix()
	return ImageID(fmt.Sprintf("%s%d:%s", buildingPrefix, now, dfSig))
}

func isBuilding(id ImageID) bool {
	return strings.HasPrefix(string(id), buildingPrefix)
}

func parseBuildingMarker(id ImageID) (time.Time, bool) {
	if !isBuilding(id) {
		return time.Time{}, false
	}
	rest := strings.TrimPrefix(string(id), buildingPrefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return time.Time{}, false
	}
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(sec, 0), true
}

func isBuildingStale(id ImageID) bool {
	ts, ok := parseBuildingMarker(id)
	if !ok {
		return false
	}
	return time.Since(ts) > buildingStaleAfter
}
