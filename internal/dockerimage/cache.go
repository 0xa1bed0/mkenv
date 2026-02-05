package dockerimage

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/0xa1bed0/mkenv/internal/dockerfile"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/0xa1bed0/mkenv/internal/state"
)

// TODO: make sensible
const (
	buildingStaleAfter = 10 * time.Minute
	buildingPrefix     = "BUILDING:" // full format: BUILDING:<unixTs>:<dfSig>
)

func (id ImageID) IsBuilding() bool {
	return strings.HasPrefix(string(id), buildingPrefix)
}

func (id ImageID) isBuildingStale() bool {
	if !id.IsBuilding() {
		return false
	}
	rest := strings.TrimPrefix(string(id), buildingPrefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return false
	}
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	return time.Since(time.Unix(sec, 0)) > buildingStaleAfter
}

func newBuildingTag(dfSig string) ImageID {
	now := time.Now().Unix()

	return ImageID(fmt.Sprintf("%s%d:%s", buildingPrefix, now, dfSig))
}

// cacheKeyDockerfileLines deterministically computes a cache key for a list of Dockerfile lines.
// It prefixes each line with its length (8-byte big-endian) before hashing to avoid collisions
// between sequences like ["ab", "c"] and ["a", "bc"].
func cacheKeyFromDockerfile(df dockerfile.Dockerfile) state.KVStoreKey {
	h := sha256.New()
	var lenBuf [8]byte

	for _, line := range df {
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(line)))
		h.Write(lenBuf[:])
		io.WriteString(h, line)
	}

	return state.KVStoreKey(hex.EncodeToString(h.Sum(nil)))
}

func cacheKeyFromProject(ctx context.Context, project *runtime.Project) state.KVStoreKey {
	signature, err := project.Signature(ctx)
	if err != nil {
		logs.Warnf("can't derive cache key from project: %v\nSkipping...", err)
		return ""
	}
	logs.Debugf("cacheKeyFromProject: path=%s, signature=%s", project.Path(), signature)
	return state.KVStoreKey(signature)
}

type DockerImageCache struct {
	kvStore *state.KVStore
}

func NewNaiveDockerImageCache(kvStore *state.KVStore) *DockerImageCache {
	// This cache is intentionally very naive,
	// because mkenv users are using this tool to write code, and not to build images very efficient.
	// It is forgedable if we run docker build again, but not if we do not spin up the environment.
	// also, keep in mind docker itself has build cache so build for image that was already built will be fast any way.
	if kvStore == nil {
		logs.Warnf("Docker image cache is disabled. Skipping cache operations...")
	}
	return &DockerImageCache{kvStore: kvStore}
}

var defaultDockerImageCache *DockerImageCache

func DefaultNaiveDockerImageCache(ctx context.Context) *DockerImageCache {
	if defaultDockerImageCache == nil {
		kvStore, err := state.DefaultKVStore(ctx)
		if err != nil {
			logs.Warnf("Error happened while instantiating default KVStore. Skipping... \n%v", err)
		}
		defaultDockerImageCache = NewNaiveDockerImageCache(kvStore)
	}

	return defaultDockerImageCache
}

func (dic *DockerImageCache) get(ctx context.Context, key state.KVStoreKey) (ImageID, bool, state.KVStoreKey) {
	if dic.kvStore == nil {
		logs.Debugf("cache.get: kvStore is nil, cache disabled")
		return "", false, key
	}

	logs.Debugf("cache.get: looking up key=%s", key)
	entry, found, err := dic.kvStore.Get(ctx, key)
	if err != nil {
		logs.Debugf("cache.get: error during lookup key=%s: %v", key, err)
		return "", false, key
	}
	if entry.Value == "" {
		logs.Debugf("cache.get: key=%s found=%v but value is empty", key, found)
		return "", false, key
	}

	imageID := ImageID(entry.Value)
	if imageID.isBuildingStale() {
		logs.Debugf("cache.get: key=%s has stale building tag, deleting", key)
		dic.delete(ctx, key)
		return "", false, key
	}

	logs.Debugf("cache.get: key=%s returning imageID=%s", key, imageID)
	return imageID, found, key
}

func (dic *DockerImageCache) delete(ctx context.Context, key state.KVStoreKey) {
	if dic.kvStore == nil {
		logs.Debugf("cache.delete: kvStore is nil, skipping")
		return
	}

	logs.Debugf("cache.delete: deleting key=%s", key)
	err := dic.kvStore.Delete(ctx, key)
	if err != nil {
		logs.Warnf("Can't delete image id from cache for this project. %v Skipping...", err)
	}
}

func (dic *DockerImageCache) set(ctx context.Context, key state.KVStoreKey, value ImageID) {
	if dic.kvStore == nil {
		logs.Debugf("cache.set: kvStore is nil, skipping")
		return
	}

	logs.Debugf("cache.set: key=%s, value=%s", key, value)
	err := dic.kvStore.Upsert(ctx, key, string(value))
	if err != nil {
		logs.Warnf("Can't upsert docker image cache. %v Skipping...", err)
	}
}

func (dic *DockerImageCache) GetByProject(ctx context.Context, project *runtime.Project) (ImageID, bool, state.KVStoreKey) {
	key := cacheKeyFromProject(ctx, project)
	if key == "" {
		return "", false, key
	}

	return dic.get(ctx, key)
}

func (dic *DockerImageCache) GetByDockerfile(ctx context.Context, df dockerfile.Dockerfile) (ImageID, bool, state.KVStoreKey) {
	key := cacheKeyFromDockerfile(df)
	if key == "" {
		return "", false, key
	}

	return dic.get(ctx, key)
}

func (dic *DockerImageCache) ClaimBuilding(ctx context.Context, rcKey state.KVStoreKey, dfKey state.KVStoreKey) string {
	buildingTag := newBuildingTag(string(dfKey))

	dic.set(ctx, dfKey, buildingTag)
	dic.set(ctx, rcKey, buildingTag)

	return string(buildingTag)
}

func (dic *DockerImageCache) StopBuilding(ctx context.Context, key state.KVStoreKey, expectedBuildTag string) {
	if dic.kvStore == nil {
		return
	}
	entry, found, _ := dic.kvStore.Get(ctx, key)
	if found && entry.Value == expectedBuildTag {
		err := dic.kvStore.Delete(ctx, key)
		if err != nil {
			logs.Warnf("Can't delete image build tag. Skipping... \n%v", err)
		}
	} else {
		logs.Warnf("Can't release building tag. the value in database does not equal to expected %s != %s", entry.Value, expectedBuildTag)
	}
}
