package cache

import (
	"encoding/json"
	"errors"
	"os"
)

type cacheState struct {
	path                     string
	UserPreferenceKeyToImage map[CacheKey]ImageID `json:"pref_to_image"`
	DockerfileKeyToImage     map[CacheKey]ImageID `json:"df_to_image"`
}

func (st *cacheState) getImageIDByUserPreferenceKey(key CacheKey) (ImageID, bool) {
	id, ok := st.UserPreferenceKeyToImage[key]
	if !ok {
		return "", false
	}

	if isBuilding(id) {
		if isBuildingStale(id) {
			// the cleanup is optional so no error propagated. 
			_ = st.cleanupUserPreferenceKey(key)
			return "", false
		}
	}

	return id, true
}

func (st *cacheState) getImageIDByDockerfileKey(key CacheKey) (ImageID, bool) {
	id, ok := st.DockerfileKeyToImage[key]
	if !ok {
		return "", false
	}

	if isBuilding(id) {
		if isBuildingStale(id) {
			// the cleanup is optional so no error propagated. 
			_ = st.cleanupDockerfileKey(key)
			return "", false
		}
	}

	return id, true
}

func (st *cacheState) cleanupUserPreferenceKey(key CacheKey) error {
	delete(st.UserPreferenceKeyToImage, key)

	return st.commit()
}

func (st *cacheState) cleanupDockerfileKey(key CacheKey) error {
	delete(st.DockerfileKeyToImage, key)
	
	return st.commit()
}

func (st *cacheState) cleanupImageID(userPreferenceKey, dockerfileKey CacheKey) error {
	delete(st.UserPreferenceKeyToImage, userPreferenceKey)
	delete(st.DockerfileKeyToImage, dockerfileKey)

	return st.commit()
}


func (st *cacheState) setUserPreferenceKey(key CacheKey, imgID ImageID) error {
	st.UserPreferenceKeyToImage[key] = imgID

	return st.commit()
}

func (st *cacheState) setImageID(userPreferenceKey, dockerfileKey CacheKey, imgID ImageID) error {
	st.UserPreferenceKeyToImage[userPreferenceKey] = imgID
	st.DockerfileKeyToImage[dockerfileKey] = imgID

	return st.commit()
}

func (st *cacheState) commit() error {
	if st.path == "" {
		// this is a readonly state 
		return nil
	}
	// TODO: do error logging
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, st.path)
}

func newEmptyCacheState(path string) *cacheState {
	return &cacheState{
		path: path,
		UserPreferenceKeyToImage: make(map[CacheKey]ImageID),
		DockerfileKeyToImage: make(map[CacheKey]ImageID),
	}
}

func newReadonlyEmptyCacheState() *cacheState {
	return newEmptyCacheState("")
}

func (c *Cache) loadState(readonly bool) (*cacheState, error) {
	data, err := os.ReadFile(c.cacheFilePath)
	path := c.cacheFilePath
	if readonly {
		path = ""
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newEmptyCacheState(path), nil
		}
		return nil, err
	}
	var st cacheState
	st.path = path
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	if st.UserPreferenceKeyToImage == nil {
		st.UserPreferenceKeyToImage = make(map[CacheKey]ImageID)
	}
	if st.DockerfileKeyToImage == nil {
		st.DockerfileKeyToImage = make(map[CacheKey]ImageID)
	}
	return &st, nil
}
