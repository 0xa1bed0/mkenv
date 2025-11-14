package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"path/filepath"
	"sort"

	"github.com/0xa1bed0/mkenv/internal/dockerfile"
)

type prefsKeyPayload struct {
	Project string                     `json:"project"`
	Prefs   dockerfile.UserPreferences `json:"prefs"`
}

func CacheKeyUserPreferences(projectAbsPath string, in *dockerfile.UserPreferences) (CacheKey, error) {
	project := filepath.Clean(projectAbsPath)

	var normalized dockerfile.UserPreferences
	if in != nil {
		normalized = normalizePrefs(*in)
	}

	payload := prefsKeyPayload{
		Project: project,
		Prefs:   normalized,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)
	return CacheKey(hex.EncodeToString(sum[:])), nil
}

func normalizePrefs(p dockerfile.UserPreferences) dockerfile.UserPreferences {
	out := p // this is intentional

	out.EnableBricks = cloneAndSortBricks(p.EnableBricks)
	out.DisableBricks = cloneAndSortBricks(p.DisableBricks)

	// normalize BricksConfigs: deep copy, non-nil maps
	if p.BricksConfigs == nil {
		out.BricksConfigs = make(map[dockerfile.BrickID]map[string]string)
	} else {
		out.BricksConfigs = make(map[dockerfile.BrickID]map[string]string, len(p.BricksConfigs))
		for id, cfg := range p.BricksConfigs {
			if cfg == nil {
				out.BricksConfigs[id] = make(map[string]string)
				continue
			}
			cp := make(map[string]string, len(cfg))
			maps.Copy(cp, cfg)
			out.BricksConfigs[id] = cp
		}
	}

	return out
}

func cloneAndSortBricks(bricks []dockerfile.BrickID) []dockerfile.BrickID {
	if len(bricks) == 0 {
		return []dockerfile.BrickID{}
	}
	tmp := make([]string, len(bricks))
	for i, b := range bricks {
		tmp[i] = string(b)
	}
	sort.Strings(tmp)
	out := make([]dockerfile.BrickID, len(tmp))
	for i, s := range tmp {
		out[i] = dockerfile.BrickID(s)
	}
	return out
}
