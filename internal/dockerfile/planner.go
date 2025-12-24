package dockerfile

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	sandboxappconfig "github.com/0xa1bed0/mkenv/internal/apps/sandbox/config"
	"github.com/0xa1bed0/mkenv/internal/bricks/systems"
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/runtime"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/ui"
)

type BuildPlan struct {
	system bricksengine.Brick
	args   map[string]string

	baseImage string

	packages []bricksengine.PackageSpec
	envs     map[string]string
	rootRun  []bricksengine.Command
	userRun  []bricksengine.Command

	fileTemplates []bricksengine.FileTemplate

	entrypoint []string
	cmd        []string

	attachInstruction []string

	cachePaths []string

	order []bricksengine.BrickID // for audit
}

// ExpandPackages asks the system brick to convert requests to concrete steps.
func (plan *BuildPlan) expandPackages() {
	if plan == nil {
		return
	}
	if plan.system == nil {
		return
	}

	mgr := plan.system.PackageManager()
	if mgr == nil {
		return
	}
	if len(plan.packages) == 0 {
		return
	}

	steps := mgr.Install(plan.packages)
	if len(steps) > 0 {
		plan.rootRun = append(plan.rootRun, steps...)
	}
}

func (plan *BuildPlan) processBrick(brick bricksengine.Brick) bricksengine.CacheFoldersPaths {
	for _, packageRequest := range brick.PackageRequests() {
		for _, packageSpec := range packageRequest.Packages {
			plan.packages = append(plan.packages, packageSpec.Clone())
		}
	}
	maps.Copy(plan.envs, brick.Envs())
	plan.rootRun = append(plan.rootRun, brick.RootRun()...)
	plan.userRun = append(plan.userRun, brick.UserRun()...)
	plan.fileTemplates = append(plan.fileTemplates, brick.FileTemplates()...)
	plan.order = append(plan.order, brick.ID())
	plan.cachePaths = append(plan.cachePaths, brick.CacheFolders()...)

	return bricksengine.CacheFoldersPaths{}
}

type Planner interface {
	Plan(ctx context.Context) (*BuildPlan, error)
}

type planner struct {
	project              *runtime.Project
	systemCandidates     map[bricksengine.BrickID]bricksengine.Brick
	entrypointCandidates map[bricksengine.BrickID]bricksengine.Brick

	systemBrick     bricksengine.Brick
	entrypointBrick bricksengine.Brick
	bricks          map[bricksengine.BrickID]bricksengine.Brick
}

func NewPlanner(project *runtime.Project) Planner {
	return &planner{
		project:              project,
		systemCandidates:     make(map[bricksengine.BrickID]bricksengine.Brick),
		entrypointCandidates: make(map[bricksengine.BrickID]bricksengine.Brick),
		bricks:               make(map[bricksengine.BrickID]bricksengine.Brick),
	}
}

func (p *planner) Plan(ctx context.Context) (*BuildPlan, error) {
	logs.Debugf("starting bricks estimation...")
	err := p.estimateBricks(ctx)
	if err != nil {
		return nil, err
	}

	logs.Debugf("processing system candidates...")
	err = p.processSystemCandidates(ctx)
	if err != nil {
		return nil, err
	}

	logs.Debugf("processing entrypoint candidates...")
	err = p.processEntrypointCandidates(ctx)
	if err != nil {
		return nil, err
	}

	logs.Debugf("compiling docker image build plan...")
	return p.buildPlan()
}

func (p *planner) buildPlan() (*BuildPlan, error) {
	if p.systemBrick == nil {
		return nil, errors.New("no System brick found")
	}

	plan := &BuildPlan{
		system:        p.systemBrick,
		packages:      []bricksengine.PackageSpec{},
		envs:          map[string]string{},
		rootRun:       []bricksengine.Command{},
		userRun:       []bricksengine.Command{},
		fileTemplates: []bricksengine.FileTemplate{},
		entrypoint:    []string{},
		cmd:           []string{},
		cachePaths:    []string{},
		// TODO: let bricks configure it and make sure system args is not overriden by bricks
		// TODO: replace this with sandboxconfig. remove args entirely.
		args: map[string]string{
			"MKENV_USERNAME":  sandboxappconfig.UserName,
			"MKENV_UID":       sandboxappconfig.UserUID,
			"MKENV_GID":       sandboxappconfig.UserGID,
			"MKENV_HOME":      sandboxappconfig.HomeFolder,
			"MKENV_LOCAL_BIN": sandboxappconfig.UserLocalBin,
		},
		order: []bricksengine.BrickID{},
	}

	plan.baseImage = p.systemBrick.BaseImage()

	plan.processBrick(p.systemBrick)

	if p.entrypointBrick != nil {
		plan.entrypoint = p.entrypointBrick.Entrypoint()
		plan.attachInstruction = p.entrypointBrick.AttachInstruction()
		plan.cmd = p.entrypointBrick.Cmd()
		p.bricks[p.entrypointBrick.ID()] = p.entrypointBrick
	}

	// deterministic brick order: sort brick IDs
	ids := make([]string, 0, len(p.bricks))
	for id := range p.bricks {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	for _, sid := range ids {
		if sid == "" {
			continue
		}
		id := bricksengine.BrickID(sid)
		brick := p.bricks[id]
		if len(brick.RootRun()) > 0 {
			runs := brick.RootRun()
			commands := make([]string, len(runs))
			for i, run := range runs {
				commands[i] = "\t" + run.String()
			}
			// TODO: make sure common brick really should not do this (think of shell bricks). and make user confirmation on this.
			logs.Warnf("common brick " + string(brick.ID()) + " tries to execute root commands: \n\n" + strings.Join(commands, "\n"))
		}
		plan.processBrick(brick)
	}

	plan.packages = uniquePackages(plan.packages)
	plan.rootRun = uniqueCommands(plan.rootRun)
	plan.userRun = uniqueCommands(plan.userRun)
	plan.fileTemplates = uniqueFileTemplates(plan.fileTemplates)
	plan.cachePaths = uniqueStrings(plan.cachePaths)

	plan.expandPackages()

	return plan, nil
}

func (p *planner) processEntrypointCandidates(ctx context.Context) error {
	if p.systemBrick == nil {
		return errors.New("please choose system brick first")
	}

	if p.systemBrick.Kinds().Contains(bricksengine.BrickKindEntrypoint) {
		p.entrypointCandidates[p.systemBrick.ID()] = p.systemBrick
	}

	if len(p.entrypointCandidates) == 0 {
		return nil
	}

	if len(p.entrypointCandidates) == 1 {
		for _, b := range p.entrypointCandidates {
			p.entrypointBrick = b
			return nil
		}
	}

	defaultEntrypoint := p.project.EnvConfig(ctx).DefaultEntrypointBrickID()
	if defaultEntrypoint != "" {
		if entrypointBrick, ok := p.entrypointCandidates[defaultEntrypoint]; ok {
			p.entrypointBrick = entrypointBrick
			return nil
		}
	}

	if len(p.entrypointCandidates) > 1 {
		prompt := "Multiple entrypoints options found while estimating environment. Please choose one."
		options := make([]ui.SelectOption, len(p.entrypointCandidates)+1) // +1 to also have "none" options
		i := 0
		for candidateBrickID, entrypointBrick := range p.entrypointCandidates {
			options[i] = logs.NewSelectOption(fmt.Sprintf("[%s] ENTRYPOINT %s CMD %s", candidateBrickID, strings.Join(entrypointBrick.Entrypoint(), " "), strings.Join(entrypointBrick.Cmd(), " ")), string(candidateBrickID))
			i++
		}
		// TODO: add entrypoint to the system brick
		options[i] = logs.NewSelectOption("None (default to the base system)", "none")
		selected, err := logs.PromptSelectOne(prompt, options)
		if err != nil {
			return err
		}

		if selected.OptionID() == "none" {
			return nil
		}

		if selected.OptionID() == "" && p.project.EnvConfig(ctx).DefaultEntrypointBrickID() == "" {
			return nil
		}

		p.entrypointBrick = p.entrypointCandidates[bricksengine.BrickID(selected.OptionID())]
	}

	return nil
}

func (p *planner) processSystemCandidates(ctx context.Context) error {
	if len(p.systemCandidates) == 0 {
		logs.Infof("no system candidates found using mkenv-default system: debian")
		platformDefaultSystemBrick, err := systems.NewDebian(nil)
		if err != nil {
			return err
		}
		p.systemBrick = platformDefaultSystemBrick
		return nil
	}

	if len(p.systemCandidates) == 1 {
		logs.Debugf("only one system candidate found. using it...")
		for _, b := range p.systemCandidates {
			p.systemBrick = b
			break
		}

		return nil
	}

	logs.Debugf("multiple system candidates found")
	prompt := "Multiple systems found while estimating environment. Please choose one."
	options := make([]ui.SelectOption, len(p.systemCandidates))
	for candidateBrickID, systemBrick := range p.systemCandidates {
		options = append(options, logs.NewSelectOption(fmt.Sprintf("[%s] %s", candidateBrickID, systemBrick.Description()), string(candidateBrickID)))
	}
	selected, err := logs.PromptSelectOne(prompt, options)
	if err != nil {
		return err
	}

	if selected.OptionID() == "" && p.project.EnvConfig(ctx).DefaultSystemBrick() == "" {
		return errors.New("must choose system brick")
	}

	p.systemBrick = p.systemCandidates[bricksengine.BrickID(selected.OptionID())]

	return nil
}

func (p *planner) estimateBricks(ctx context.Context) error {
	enabledBricks := p.project.EnvConfig(ctx).EnableBricks()
	if p.project.EnvConfig(ctx).DefaultEntrypointBrickID() != "" {
		enabledBricks = append(enabledBricks, p.project.EnvConfig(ctx).DefaultEntrypointBrickID())
	}

	forceEnabled := bricksengine.ToSet(enabledBricks)
	forceDsiabled := bricksengine.ToSet(p.project.EnvConfig(ctx).DisableBricks())

	add := func(b bricksengine.Brick, reason string) {
		id := b.ID()
		if forceDsiabled[id] {
			logs.Warnf("skipping disabled brick %s (reason: %s)", id, reason)
			return
		}

		if b.Kinds().Contains(bricksengine.BrickKindSystem) {
			// since we should completely discard non selected systems we can't add system bricks to the main bricks map.
			// we will add single chosen system as entrypoint candidate later when we discard the rest
			p.systemCandidates[id] = b
			return
		}

		if b.Kinds().Contains(bricksengine.BrickKindEntrypoint) {
			p.entrypointCandidates[id] = b
		}

		if _, ok := p.bricks[id]; !ok {
			p.bricks[id] = b
		}
	}

	for _, id := range enabledBricks {
		if factory, ok := bricksengine.DefaultBricksRegistry.GetBrickFactory(id); ok {
			b, err := factory(p.project.EnvConfig(ctx).BricksConfigs()[id])
			if err != nil {
				return err
			}
			add(b, "enabled by user settings")
		} else {
			logs.Warnf("Unknown brick '%s'. Skipping...", id)
		}
	}

	if !p.project.EnvConfig(ctx).ShouldDisableAuto() {
		detectors := bricksengine.DefaultBricksRegistry.AllDetectors()
		for _, d := range detectors {
			if mentionsAny(d.BrickInfo().ID(), forceEnabled, forceDsiabled) {
				continue // we already added forceEnabled bricks and we don't want to iterate over forceDisabled
			}

			folderPtr, err := p.project.FolderPtr()
			if err != nil {
				logs.Errorf("Can't scan project files: %v", err)
				return err
			}

			id, meta, err := d.Scan(folderPtr)
			if err != nil {
				return err
			}
			if id == "" {
				continue
			}

			if factory, ok := bricksengine.DefaultBricksRegistry.GetBrickFactory(id); ok {
				b, err := factory(meta)
				if err != nil {
					return err
				}

				add(b, "proposed by detector")
			} else {
				logs.Warnf("Detector proposed unknown brick %s", id)
			}
		}
	}

	return nil
}

func mentionsAny(id bricksengine.BrickID, en, dis map[bricksengine.BrickID]bool) bool {
	return en[id] || dis[id]
}

func uniquePackages(specs []bricksengine.PackageSpec) []bricksengine.PackageSpec {
	if len(specs) == 0 {
		return specs
	}
	seen := make(map[string]struct{}, len(specs))
	out := make([]bricksengine.PackageSpec, 0, len(specs))
	for _, spec := range specs {
		key := packageKey(spec)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, spec)
	}
	return out
}

func packageKey(spec bricksengine.PackageSpec) string {
	if len(spec.Meta) == 0 {
		return spec.Name
	}
	keys := make([]string, 0, len(spec.Meta))
	for k := range spec.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(spec.Name)
	for _, k := range keys {
		b.WriteString("\x1f")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(spec.Meta[k])
	}
	return b.String()
}

func uniqueCommands(cmds []bricksengine.Command) []bricksengine.Command {
	if len(cmds) == 0 {
		return cmds
	}
	seen := make(map[string]struct{}, len(cmds))
	out := make([]bricksengine.Command, 0, len(cmds))
	for _, cmd := range cmds {
		key := cmd.When + "\x1f" + strings.Join(cmd.Argv, "\x1f")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, cmd)
	}
	return out
}

func uniqueFileTemplates(templates []bricksengine.FileTemplate) []bricksengine.FileTemplate {
	if len(templates) == 0 {
		return templates
	}
	seen := make(map[string]struct{}, len(templates))
	out := make([]bricksengine.FileTemplate, 0, len(templates))
	for _, tmpl := range templates {
		key := tmpl.ID + "\x1f" + tmpl.FilePath + "\x1f" + tmpl.Content
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tmpl)
	}
	return out
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
