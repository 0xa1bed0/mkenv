package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/guardrails"
	"github.com/0xa1bed0/mkenv/internal/logs"
)

type EnvConfig interface {
	EnableBricks() []bricksengine.BrickID
	DisableBricks() []bricksengine.BrickID
	BricksConfigs() map[bricksengine.BrickID]map[string]string
	BrickConfig(bricksengine.BrickID) map[string]string
	DefaultEntrypointBrickID() bricksengine.BrickID
	DefaultSystemBrick() bricksengine.BrickID
	ShouldDisableAuto() bool
	Volumes() []string

	FilePath() string           // path to .mkenv file that correspond to this env config
	Signature() (string, error) // return signature of the object
}

type envConfig struct {
	name                      string
	EnableBricks_             []bricksengine.BrickID                     `json:"enabled_bricks"`
	DisableBricks_            []bricksengine.BrickID                     `json:"disabled_bricks"`
	BricksConfigs_            map[bricksengine.BrickID]map[string]string `json:"bricks_config"`
	DefaultEntrypointBrickID_ bricksengine.BrickID                       `json:"entrypoint"`
	DefaultSystemBrick_       bricksengine.BrickID                       `json:"system"`
	ShouldDisableAuto_        bool                                       `json:"disable_auto"`
	Volumes_                  []string                                   `json:"volumes"`
}

func (ec envConfig) Copy() *envConfig {
	newEncConfig := buildDefaultEnvConfig()
	newEncConfig.name = ec.name
	newEncConfig.EnableBricks_ = bricksengine.CopyBrickIDs(ec.EnableBricks_)
	newEncConfig.DisableBricks_ = bricksengine.CopyBrickIDs(ec.DisableBricks_)
	newEncConfig.BricksConfigs_ = map[bricksengine.BrickID]map[string]string{}
	for brickID, brickConfig := range ec.BricksConfigs_ {
		newEncConfig.BricksConfigs_[brickID] = map[string]string{}
		for k, v := range brickConfig {
			newEncConfig.BricksConfigs_[brickID][k] = v
		}
	}
	newEncConfig.DefaultSystemBrick_ = ec.DefaultSystemBrick_
	newEncConfig.DefaultEntrypointBrickID_ = ec.DefaultEntrypointBrickID_
	newEncConfig.ShouldDisableAuto_ = ec.ShouldDisableAuto_
	newEncConfig.Volumes_ = []string{}
	for _, vol := range ec.Volumes_ {
		newEncConfig.Volumes_ = append(newEncConfig.Volumes_, vol)
	}
	return newEncConfig
}

func (ec *envConfig) Signature() (string, error) {
	// TODO: normalize before signature calculation

	ecCopy := ec.Copy()

	// should not be included to sugnature because signature is a part of docker image cache key.
	// TODO: move this whole signature function to the state package. it should not be here
	ecCopy.Volumes_ = []string{}

	data, err := json.Marshal(ecCopy)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

type envConfigOption func(*envConfig)

func WithEnableBricks(bricks []bricksengine.BrickID) envConfigOption {
	return func(rc *envConfig) {
		rc.EnableBricks_ = bricksengine.UniqueSortedBricks(bricks)
	}
}

func WithDisableBricks(bricks []bricksengine.BrickID) envConfigOption {
	return func(rc *envConfig) {
		rc.DisableBricks_ = bricksengine.UniqueSortedBricks(bricks)
	}
}

func WithBricksConfigs(cfgs map[bricksengine.BrickID]map[string]string) envConfigOption {
	return func(rc *envConfig) {
		rc.BricksConfigs_ = make(map[bricksengine.BrickID]map[string]string, len(cfgs))
		for k, inner := range cfgs {
			outInner := make(map[string]string, len(inner))
			maps.Copy(outInner, inner)
			rc.BricksConfigs_[k] = outInner
		}
	}
}

func WithDefaultEntrypointBrickID(brickID bricksengine.BrickID) envConfigOption {
	return func(rc *envConfig) {
		if brickID != "" {
			rc.DefaultEntrypointBrickID_ = brickID
		}
	}
}

func WithDefaultSystemBrickID(brickID bricksengine.BrickID) envConfigOption {
	return func(rc *envConfig) {
		if brickID != "" {
			rc.DefaultSystemBrick_ = brickID
		}
	}
}

func WithDisabledAuto() envConfigOption {
	return func(rc *envConfig) {
		rc.ShouldDisableAuto_ = true
	}
}

func BuildEnvConfig(opts ...envConfigOption) EnvConfig {
	cfg := buildDefaultEnvConfig()
	cfg.name = "built-inmemmory"

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

func buildDefaultEnvConfig() *envConfig {
	return &envConfig{
		name:                      "calculated",
		EnableBricks_:             []bricksengine.BrickID{},
		DisableBricks_:            []bricksengine.BrickID{},
		BricksConfigs_:            map[bricksengine.BrickID]map[string]string{},
		DefaultEntrypointBrickID_: "",
		DefaultSystemBrick_:       "",
		ShouldDisableAuto_:        false,
		Volumes_:                  []string{},
	}
}

func (ec *envConfig) BrickConfig(brickID bricksengine.BrickID) map[string]string {
	cfg, ok := ec.BricksConfigs_[brickID]
	if !ok {
		return make(map[string]string)
	}
	return maps.Clone(cfg)
}

func (ec *envConfig) Merge(src EnvConfig) {
	if src == nil {
		return
	}

	ec.EnableBricks_ = bricksengine.UniqueSortedBricks(append(ec.EnableBricks_, src.EnableBricks()...))
	for _, brick := range src.EnableBricks() {
		logs.Debugf("Brick %s enabled by %s", brick, src.FilePath())
	}

	ec.DisableBricks_ = bricksengine.UniqueSortedBricks(append(ec.DisableBricks_, src.DisableBricks()...))
	for _, brick := range src.DisableBricks() {
		logs.Debugf("Brick %s disabled by %s", brick, src.FilePath())
	}

	srcConfigs := src.BricksConfigs()
	for brick, cfg := range srcConfigs {
		existing := srcConfigs[brick]
		if existing == nil {
			existing = make(map[string]string)
			ec.BricksConfigs_[brick] = existing
		}
		for k, v := range cfg {
			existing[k] = v
			logs.Debugf("Brick %s configuration key %v is set to value %s by %s", brick, k, v, src.FilePath())
		}
	}

	if src.DefaultSystemBrick() != "" {
		ec.DefaultSystemBrick_ = src.DefaultSystemBrick()
		logs.Debugf("default system is set to %s by %s", ec.DefaultSystemBrick_, src.FilePath())
	}

	if src.DefaultEntrypointBrickID() != "" {
		ec.DefaultEntrypointBrickID_ = src.DefaultEntrypointBrickID()
		logs.Debugf("default entrypoint is set to %s by %s", ec.DefaultEntrypointBrickID_, src.FilePath())
	}

	ec.ShouldDisableAuto_ = ec.ShouldDisableAuto_ || src.ShouldDisableAuto()
	if src.ShouldDisableAuto() {
		logs.Debugf("environment auto estimation is disabled by %s", src.FilePath())
	}

	ec.Volumes_ = append(ec.Volumes_, src.Volumes()...)
}

func (ec *envConfig) FilePath() string {
	return ec.name
}

func (ec *envConfig) Volumes() []string {
	out := []string{}
	out = append(out, ec.Volumes_...)
	return out
}

func (ec *envConfig) EnableBricks() []bricksengine.BrickID {
	return bricksengine.CopyBrickIDs(ec.EnableBricks_)
}

func (ec *envConfig) DisableBricks() []bricksengine.BrickID {
	return bricksengine.CopyBrickIDs(ec.DisableBricks_)
}

func (ec *envConfig) BricksConfigs() map[bricksengine.BrickID]map[string]string {
	out := make(map[bricksengine.BrickID]map[string]string, len(ec.BricksConfigs_))

	for brickId, config := range ec.BricksConfigs_ {
		out[brickId] = make(map[string]string, len(config))
		for k, v := range config {
			out[brickId][k] = v
		}
	}

	return out
}

func (ec *envConfig) DefaultEntrypointBrickID() bricksengine.BrickID {
	return ec.DefaultEntrypointBrickID_
}

func (ec *envConfig) DefaultSystemBrick() bricksengine.BrickID {
	return ec.DefaultSystemBrick_
}

func (ec *envConfig) ShouldDisableAuto() bool {
	return ec.ShouldDisableAuto_
}

func ensureProjectPathIsSafe(ctx context.Context, policy guardrails.Policy, project *Project) error {
	projectPath := project.Path()

	if guardrails.IsAbsolutelyForbidden(projectPath) {
		return fmt.Errorf("project path %s is not allowed by mkenv", projectPath)
	}

	if len(policy.AllowedProjectRoot()) > 0 && !guardrails.IsUnderPrefix(policy.AllowedProjectRoot(), projectPath) {
		return fmt.Errorf("project path %s is rejected by policy. Path is not under allowed projects root %s", projectPath, policy.AllowedProjectRoot())
	}

	if !project.Known() {
		logs.Infof("Project %s is not known. Scanning files to ensure it's safeness...", projectPath)
		warnings, err := guardrails.ScanSuspiciousFiles(ctx, projectPath)
		if err != nil {
			return err
		}

		if len(warnings) > 0 {
			text := "It looks like the project folder contain potentially sensitive files. If you continue they will be mounted to the sandbox: \n\n"
			for _, warn := range warnings {
				text += "\n\t" + "- " + warn.Path + " - " + warn.Reason + "\n"
				if len(warn.Content) > 0 {
					text += "\n"
				}
				for _, line := range warn.Content {
					text += "\t\t" + line + "\n"
				}
				text += "\n"
			}
			text += "\n"
			text += "It looks like the project folder contain potentially sensitive files. If you continue they will be mounted to the sandbox. Please scroll up and review all of them before confirming\n"
			ok, err := logs.PromptConfirm(text)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("user prompt failed")
			}
		}
	}

	return nil
}

func resolvePreferencesChain(projectPath string) ([]string, error) {
	dir := projectPath
	info, err := os.Stat(projectPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		dir = filepath.Dir(projectPath)
	}

	var dirs []string
	for {
		dirs = append(dirs, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root
		}
		dir = parent
	}

	// Walked from leaf → root. Reverse to root → leaf.
	for i, j := 0, len(dirs)-1; i < j; i, j = i+1, j-1 {
		dirs[i], dirs[j] = dirs[j], dirs[i]
	}

	var files []string
	for _, d := range dirs {
		// TODO: put .mkenv filename to constants
		f := filepath.Join(d, ".mkenv")
		if _, err := os.Stat(f); err == nil {
			logs.Infof("Loading .mkenv file at %s ...", f)
			files = append(files, f)
		}
	}
	return files, nil
}

func loadPreferencesFile(path string) (*envConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p envConfig
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse preferences %s: %w", path, err)
	}
	if p.BricksConfigs_ == nil {
		p.BricksConfigs_ = make(map[bricksengine.BrickID]map[string]string)
	}
	p.name = path
	return &p, nil
}

func applyPolicy(rc *envConfig, policy guardrails.Policy) error {
	if len(policy.EnableBricks()) > 0 {
		rc.EnableBricks_ = bricksengine.UniqueSortedBricks(append(rc.EnableBricks_, policy.EnableBricks()...))
		for _, brick := range policy.EnableBricks() {
			logs.Debugf("Brick %s enabled by policy", brick)
		}
	}

	if len(policy.DisableBricks()) > 0 {
		rc.DisableBricks_ = bricksengine.UniqueSortedBricks(append(rc.DisableBricks_, policy.DisableBricks()...))
		for _, brick := range policy.DisableBricks() {
			logs.Debugf("Brick %s disabled by policy", brick)
		}
	}

	if policy.DisableAuto() {
		rc.ShouldDisableAuto_ = true
		logs.Debugf("environment auto-estimation disabled by policy")
	}

	if len(policy.AllowedMounts()) > 0 {
		errors := []error{}
		for brick, cfg := range rc.BricksConfigs_ {
			for k, v := range cfg {
				if k != "mounts" {
					continue
				}
				if len(v) == 0 {
					continue
				}
				brickMounts := strings.Split(v, ",")
				if len(brickMounts) == 0 {
					continue
				}
				for _, p := range brickMounts {
					allowed := false
					if guardrails.IsAbsolutelyForbidden(p) {
						allowed = false
						break
					}
					for _, allowedMount := range policy.AllowedMounts() {
						if guardrails.IsUnderPrefix(allowedMount, p) {
							allowed = true
							break
						}
					}
					if !allowed {
						errors = append(errors, fmt.Errorf("mount path request from %s does not allowed by policy. (path %s is not under any allowed paths)", brick, p))
					}
				}
			}
		}
		for _, vol := range rc.Volumes_ {
			binds := strings.Split(vol, ":")
			allowed := false
			if guardrails.IsAbsolutelyForbidden(binds[0]) {
				allowed = false
				break
			}
			for _, allowedMount := range policy.AllowedMounts() {
				if guardrails.IsUnderPrefix(allowedMount, binds[0]) {
					allowed = true
					break
				}
			}
			if !allowed {
				errors = append(errors, fmt.Errorf("mount path request from EnvConfig does not allowed by policy. (path %s is not under any allowed paths)", binds[0]))
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("%v", errors)
		}
	}

	return nil
}

func (p *Project) SetEnvConfigOverride(ec EnvConfig) {
	// TODO: should we check if the env config already resolved?
	// it won't take effect if env config resolved already
	p.envConfigOverride = ec
}

func (p *Project) resolveEnvConfig(ctx context.Context) error {
	policy, err := guardrails.LoadPolicy()
	if err != nil {
		return err
	}

	err = ensureProjectPathIsSafe(ctx, policy, p)
	if err != nil {
		return err
	}

	prefsChain, err := resolvePreferencesChain(p.Path())
	if err != nil {
		return err
	}

	envCfg := buildDefaultEnvConfig()
	// TODO: restrict volumes config in project .mkenv -
	// there is absolutely zero legit reasons to let project deside which folders to mount other than project folder
	// which is automatically mounted
	for _, prefPath := range prefsChain {
		pref, errLoadPrefs := loadPreferencesFile(prefPath)
		if errLoadPrefs != nil {
			return errLoadPrefs
		}

		envCfg.Merge(pref)
	}

	if p.envConfigOverride != nil {
		envCfg.Merge(p.envConfigOverride)
	}

	err = applyPolicy(envCfg, policy)
	if err != nil {
		return err
	}

	if !p.Known() {
		ok, err := logs.PromptConfirm("You run this project for the first time. Continue?")
		if err != nil {
			return err
		}
		if !ok {
			// TODO: panic?
			os.Exit(1)
		}
		p.SetKnown(ctx)
	}

	p.envConfig = envCfg

	return nil
}
