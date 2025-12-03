package guardrails

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

type policy struct {
	DisableBricks_      []bricksengine.BrickID                     `json:"disabled_bricks"`
	EnableBricks_       []bricksengine.BrickID                     `json:"enabled_bricks"`
	DisableAuto_        bool                                       `json:"disable_auto"`
	BricksConfigs_      map[bricksengine.BrickID]map[string]string `json:"bricks_config"`
	AllowedMounts_      []string                                   `json:"allowed_mount_paths"`  // if empty - allow all except forbidden globally
	AllowedProjectRoot_ string                                     `json:"allowed_project_path"` // if empty - allow all except forbidden globally
	IgnorePreferences_  bool                                       `json:"ignore_preferences"`
}

// AllowedMounts implements Policy.
func (p *policy) AllowedMounts() []string {
	out := make([]string, len(p.AllowedMounts_))
	for i, val := range p.AllowedMounts_ {
		out[i] = val
	}
	return out
}

// AllowedProjectRoot implements Policy.
func (p *policy) AllowedProjectRoot() string {
	return p.AllowedProjectRoot_
}

// BricksConfigs implements Policy.
func (p *policy) BricksConfigs() map[bricksengine.BrickID]map[string]string {
	out := make(map[bricksengine.BrickID]map[string]string, len(p.BricksConfigs_))

	for brickId, config := range p.BricksConfigs_ {
		out[brickId] = make(map[string]string, len(config))
		for k, v := range config {
			out[brickId][k] = v
		}
	}

	return out
}

// DisableAuto implements Policy.
func (p *policy) DisableAuto() bool {
	return p.DisableAuto_
}

// DisableBricks implements Policy.
func (p *policy) DisableBricks() []bricksengine.BrickID {
	return bricksengine.CopyBrickIDs(p.DisableBricks_)
}

// EnableBricks implements Policy.
func (p *policy) EnableBricks() []bricksengine.BrickID {
	return bricksengine.CopyBrickIDs(p.EnableBricks_)
}

// IgnorePreferences implements Policy.
func (p *policy) IgnorePreferences() bool {
	return p.IgnorePreferences_
}

type Policy interface {
	DisableBricks() []bricksengine.BrickID
	EnableBricks() []bricksengine.BrickID
	DisableAuto() bool
	BricksConfigs() map[bricksengine.BrickID]map[string]string
	AllowedMounts() []string
	AllowedProjectRoot() string
	IgnorePreferences() bool
}

var defaultPolicy = policy{
	DisableBricks_:      []bricksengine.BrickID{},
	EnableBricks_:       []bricksengine.BrickID{},
	DisableAuto_:        false,
	BricksConfigs_:      map[bricksengine.BrickID]map[string]string{},
	AllowedMounts_:      []string{},
	AllowedProjectRoot_: "",
	IgnorePreferences_:  false,
}

func ensurePolicyLocked(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no policy → fine
		}
		return err
	}

	// Check permission bits only.
	if info.Mode().Perm() != 0o444 {
		return fmt.Errorf("policy file %s must have permissions 0444 (read-only), but has %04o", path, info.Mode().Perm())
	}
	return nil
}

func LoadPolicy() (Policy, error) {
	policyPath, _ := filepath.Abs(hostappconfig.ConfigBasePath() + "policy.json")
	if err := ensurePolicyLocked(policyPath); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(policyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &defaultPolicy, nil
		}
		return nil, err
	}
	var p policy
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
