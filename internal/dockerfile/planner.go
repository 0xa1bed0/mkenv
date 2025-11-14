package dockerfile

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

// UserPreferences is user overides for environment
type UserPreferences struct {
	EnableBricks      []BrickID
	DisableBricks     []BrickID
	BricksConfigs     map[BrickID]map[string]string
	DisableAuto       bool
	EntrypointBrickId BrickID
	SystemBrickId     BrickID
}

type BuildPlan struct {
	system Brick
	args   map[string]string

	baseImage string

	packages []PackageSpec
	envs     map[string]string
	rootRun  []Command
	userRun  []Command

	fileTemplates []FileTemplate

	entrypoint []string
	cmd        []string

	cachePaths []string

	order []BrickID // for audit
}

func (plan *BuildPlan) processBrick(brick Brick) CacheFoldersPaths {
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

	return CacheFoldersPaths{}
}

type PlanResult struct {
	BuildPlan *BuildPlan
	Err       error
}

type UserPrompt interface {
	isUserPrompt()
}

type UserInputRequest[T comparable] struct {
	Key     string
	Prompt  string
	Options map[T]string
	Default T
}

func (UserInputRequest[T]) isUserPrompt() {}

type UserInputResponse[T comparable] struct {
	Key     string
	Reponse T
}

func (UserInputResponse[T]) isUserPrompt() {}

type Warning struct {
	Msg string
}

func (Warning) isUserPrompt() {}

type Planner interface {
	Plan(ctx context.Context) <-chan PlanResult
	UserPromptsChan() <-chan UserPrompt
	UserPromptsResponsesChan() chan<- UserInputResponse[any]
}

type planner struct {
	userPreferences      *UserPreferences
	folderPtr            filesmanager.FileManager
	userPrompts          chan UserPrompt
	userResponses        chan UserInputResponse[any]
	systemCandidates     map[BrickID]Brick
	entrypointCandidates map[BrickID]Brick

	systemBrick     Brick
	entrypointBrick Brick
	bricks          map[BrickID]Brick
}

func NewPlanner(folderPtr filesmanager.FileManager, userPreferences *UserPreferences) Planner {
	return &planner{
		userPreferences:      userPreferences,
		folderPtr:            folderPtr,
		userPrompts:          make(chan UserPrompt, 1),
		userResponses:        make(chan UserInputResponse[any], 1),
		systemCandidates:     make(map[BrickID]Brick),
		entrypointCandidates: make(map[BrickID]Brick),
		bricks:               make(map[BrickID]Brick),
	}
}

func (p *planner) UserPromptsChan() <-chan UserPrompt {
	return p.userPrompts
}

func (p *planner) UserPromptsResponsesChan() chan<- UserInputResponse[any] {
	return p.userResponses
}

func (p *planner) Plan(ctx context.Context) <-chan PlanResult {
	ch := make(chan PlanResult, 1)

	go func() {
		defer close(p.userPrompts)
		defer close(ch)
		var out PlanResult
		var err error

		err = p.estimateBricks(ctx)
		if err != nil {
			out.Err = err
			ch <- out
			return
		}

		err = p.processSystemCandidates(ctx)
		if err != nil {
			out.Err = err
			ch <- out
			return
		}

		err = p.processEntrypointCandidates(ctx)
		if err != nil {
			out.Err = err
			ch <- out
			return
		}

		plan, err := p.buildPlan()
		if err != nil {
			out.Err = err
			ch <- out
			return
		}

		out.BuildPlan = plan

		ch <- out
	}()

	return ch
}

func (p *planner) buildPlan() (*BuildPlan, error) {
	if p.systemBrick == nil {
		return nil, errors.New("no System brick found")
	}

	plan := &BuildPlan{
		system:        p.systemBrick,
		packages:      []PackageSpec{},
		envs:          map[string]string{},
		rootRun:       []Command{},
		userRun:       []Command{},
		fileTemplates: []FileTemplate{},
		entrypoint:    []string{},
		cmd:           []string{},
		cachePaths:    []string{},
		// TODO: let bricks configure it and make sure system args is not overriden by bricks
		args: map[string]string{
			"MKENV_USERNAME":  "dev",
			"MKENV_UID":       "10000",
			"MKENV_GID":       "10000",
			"MKENV_HOME":      "/home/dev",
			"MKENV_LOCAL_BIN": "/home/dev/local/bin",
		},
		order: []BrickID{},
	}

	plan.baseImage = p.systemBrick.BaseImage()

	plan.processBrick(p.systemBrick)

	if p.entrypointBrick != nil {
		plan.entrypoint = p.entrypointBrick.Entrypoint()
		plan.cmd = p.entrypointBrick.Cmd()
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
		id := BrickID(sid)
		brick := p.bricks[id]
		if len(brick.RootRun()) > 0 {
			runs := brick.RootRun()
			commands := make([]string, len(runs))
			for i, run := range runs {
				commands[i] = "\t" + run.String()
			}
			// TODO: implement option to stop and wait for confirmation
			p.userPrompts <- &Warning{Msg: "common brick " + string(brick.ID()) + " tries to execute root commands: \n\n" + strings.Join(commands, "\n")}
		}
		plan.processBrick(brick)
	}

	plan.expandPackages()

	return plan, nil
}

func (p *planner) processEntrypointCandidates(ctx context.Context) error {
	if p.systemBrick == nil {
		return errors.New("please choose system brick first")
	}

	if p.systemBrick.Kinds().Contains(BrickKindEntrypoint) {
		p.entrypointCandidates[p.systemBrick.ID()] = p.systemBrick
	}

	if len(p.entrypointCandidates) == 0 {
		return nil
	}

	if len(p.entrypointCandidates) == 1 {
		for _, b := range p.entrypointCandidates {
			p.entrypointBrick = b
			break
		}
	}

	if len(p.entrypointCandidates) > 1 {
		prompt := "Multiple entrypoints options found while estimating environment. Please choose one."
		options := make(map[BrickID]string, len(p.entrypointCandidates)+1) // +1 to also have "none" options
		for candidateBrickID, entrypointBrick := range p.entrypointCandidates {
			options[candidateBrickID] = fmt.Sprintf("[%s] ENTRYPOINT %s CMD %s", candidateBrickID, strings.Join(entrypointBrick.Entrypoint(), " "), strings.Join(entrypointBrick.Cmd(), " "))
		}
		options["none"] = "None (default to the base system)"
		id, err := askUser(ctx, p.userPrompts, p.userResponses, &UserInputRequest[BrickID]{
			Key:     "choose_entrypoint",
			Prompt:  prompt,
			Options: options,
			Default: p.userPreferences.EntrypointBrickId,
		})
		if err != nil {
			return err
		}

		if id == "none" {
			return nil
		}

		if id == "" && p.userPreferences.EntrypointBrickId == "" {
			return nil
		}

		p.entrypointBrick = p.entrypointCandidates[id]
	}

	return nil
}

func (p *planner) processSystemCandidates(ctx context.Context) error {
	if len(p.systemCandidates) == 0 {
		return errors.New("Can't build environment without base system.")
	}

	if len(p.systemCandidates) == 1 {
		for _, b := range p.systemCandidates {
			p.systemBrick = b
			break
		}

		return nil
	}

	if len(p.systemCandidates) > 1 {
		prompt := "Multiple systems found while estimating environment. Please choose one."
		options := make(map[BrickID]string, len(p.systemCandidates))
		for candidateBrickID, systemBrick := range p.systemCandidates {
			options[candidateBrickID] = fmt.Sprintf("[%s] %s", candidateBrickID, systemBrick.Description())
		}
		id, err := askUser(ctx, p.userPrompts, p.userResponses, &UserInputRequest[BrickID]{
			Key:     "choose_system",
			Prompt:  prompt,
			Options: options,
			Default: p.userPreferences.SystemBrickId,
		})
		if err != nil {
			return err
		}

		if id == "" && p.userPreferences.SystemBrickId == "" {
			return errors.New("Must choose system brick.")
		}

		p.systemBrick = p.systemCandidates[id]
	}

	return nil
}

func askUser[T comparable](ctx context.Context, requestCh chan UserPrompt, responseCh chan UserInputResponse[any], prompt *UserInputRequest[T]) (T, error) {
	var zero T

	select {
	case requestCh <- prompt:
	case <-ctx.Done():
		return zero, ctx.Err()
	}

	for {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case resp := <-responseCh:
			if resp.Key != prompt.Key {
				continue
			}
			if resp.Reponse == nil {
				return prompt.Default, nil
			}
			v, ok := resp.Reponse.(T)
			if !ok {
				return zero, fmt.Errorf("invalid response type for %s", prompt.Key)
			}
			return v, nil
		}
	}
}

func (p *planner) estimateBricks(ctx context.Context) error {
	// TODO: maybe we just need to add those to candidates and not enable straight away
	enabledBricks := append(p.userPreferences.EnableBricks, p.userPreferences.SystemBrickId, p.userPreferences.EntrypointBrickId)

	forceEnabled := toSet(enabledBricks)
	forceDsiabled := toSet(p.userPreferences.DisableBricks)

	add := func(b Brick, reason string) {
		id := b.ID()
		if forceDsiabled[id] {
			p.userPrompts <- &Warning{Msg: fmt.Sprintf("skipping disabled brick %s (reason: %s)", id, reason)}
			return
		}

		if b.Kinds().Contains(BrickKindSystem) {
			// since we should completely discard non selected systems we can't add system bricks to the main bricks map.
			// we will add single chosen system as entrypoint candidate later when we discard the rest
			p.systemCandidates[id] = b
			return
		}

		if b.Kinds().Contains(BrickKindEntrypoint) {
			p.entrypointCandidates[id] = b
		}

		if _, ok := p.bricks[id]; !ok {
			p.bricks[id] = b
		}
	}

	for _, id := range enabledBricks {
		if id == "" {
			continue // TODO: fix enabledBricks composition
		}
		if factory, ok := DefaultBricksRegistry.GetBrickFactory(id); ok {
			b, err := factory(p.userPreferences.BricksConfigs[id])
			if err != nil {
				return err
			}
			add(b, "enabled by user settings")
		} else {
			p.userPrompts <- &Warning{Msg: fmt.Sprintf("unknown brick id = \"%s\". skipping...", id)}
		}
	}

	if !p.userPreferences.DisableAuto {
		detectors := DefaultBricksRegistry.AllDetectors()
		for _, d := range detectors {
			if mentionsAny(d.BrickInfo().id, forceEnabled, forceDsiabled) {
				continue // we already added forceEnabled bricks and we don't want to iterate over forceDisabled
			}

			id, meta, err := d.Scan(p.folderPtr)
			if err != nil {
				return err
			}
			if id == "" {
				continue
			}

			if factory, ok := DefaultBricksRegistry.GetBrickFactory(id); ok {
				b, err := factory(meta)
				if err != nil {
					return err
				}

				add(b, "proposed by detector")
			} else {
				p.userPrompts <- &Warning{Msg: fmt.Sprintf("Detector proposed unknown brick %s", id)}
			}
		}
	}

	return nil
}

func toSet(xs []BrickID) map[BrickID]bool {
	m := map[BrickID]bool{}
	for _, x := range xs {
		if x != "" {
			m[x] = true
		}
	}
	return m
}

func mentionsAny(id BrickID, en, dis map[BrickID]bool) bool {
	return en[id] || dis[id]
}
