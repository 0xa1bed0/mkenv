package dockerfile

import (
	"fmt"
	"maps"
	"strings"
)

type (
	BrickID           string
	CacheFoldersPaths []string
)

type Brick interface {
	ID() BrickID
	Description() string
	Kinds() BrickKindsSet

	BaseImage() string
	PackageRequests() []PackageRequest
	Envs() map[string]string
	RootRun() []Command
	UserRun() []Command

	FileTemplates() []FileTemplate

	PackageManager() PackageManager

	CacheFolders() CacheFoldersPaths

	Entrypoint() []string
	Cmd() []string
}

type Command struct {
	When string   // currently only "build" supported
	Argv []string // e.g. []string{"/bin/sh", "-lc", "echo hi"}
}

func (c *Command) String() string {
	return fmt.Sprintf("[%s time]: %s", c.When, strings.Join(c.Argv, " "))
}

type FileTemplate struct {
	ID       string
	FilePath string
	Content  string
}

type BrickInfo struct {
	id          BrickID
	description string
	kinds       BrickKindsSet
}

func (b *BrickInfo) ID() BrickID          { return b.id }
func (b *BrickInfo) Description() string  { return b.description }
func (b *BrickInfo) Kinds() BrickKindsSet { return b.kinds.Clone() }

func NewBrickInfo(id BrickID, description string, kinds []BrickKind) *BrickInfo {
	return &BrickInfo{
		id:          id,
		description: description,
		kinds:       NewBrickKindsSet(kinds...),
	}
}

type brick struct {
	BrickInfo

	baseImage       string
	packageRequests []PackageRequest
	envs            map[string]string
	rootRun         []Command
	userRun         []Command

	fileTemplates []FileTemplate

	packageManager PackageManager

	cacheFolders CacheFoldersPaths

	entrypoint []string
	cmd        []string
}

func (b *brick) BaseImage() string       { return b.baseImage }
func (b *brick) Envs() map[string]string { return copyMap(b.envs) }
func (b *brick) RootRun() []Command      { return copyCommands(b.rootRun) }
func (b *brick) UserRun() []Command      { return copyCommands(b.userRun) }
func (b *brick) FileTemplates() []FileTemplate {
	return copyFragments(b.fileTemplates)
}

func (b *brick) PackageRequests() []PackageRequest {
	return copyPackageRequests(b.packageRequests)
}

func (b *brick) CacheFolders() CacheFoldersPaths { return copyStrings(b.cacheFolders) }
func (b *brick) PackageManager() PackageManager  { return b.packageManager }
func (b *brick) Entrypoint() []string            { return copyStrings(b.entrypoint) }
func (b *brick) Cmd() []string                   { return copyStrings(b.cmd) }

// Safe-copy helpers
func copyPackageRequests(r []PackageRequest) []PackageRequest {
	out := make([]PackageRequest, len(r))
	for _, request := range r {
		out = append(out, request.Clone())
	}
	return out
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	out := make(map[string]string, len(m))
	maps.Copy(out, m)

	return out
}

func copyCommands(src []Command) []Command {
	if src == nil {
		return nil
	}

	out := make([]Command, len(src))
	copy(out, src)

	return out
}

func copyFragments(src []FileTemplate) []FileTemplate {
	if src == nil {
		return nil
	}

	out := make([]FileTemplate, len(src))
	copy(out, src)

	return out
}

func copyStrings(src []string) []string {
	if src == nil {
		return nil
	}
	out := make([]string, len(src))

	copy(out, src)

	return out
}
