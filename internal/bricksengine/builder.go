package bricksengine

import (
	"fmt"
)

type BrickOption func(*brick) error

func NewBrick(id BrickID, description string, opts ...BrickOption) (Brick, error) {
	brick := &brick{
		BrickInfo: BrickInfo{
			id:          id,
			description: description,
			kinds:       NewBrickKindsSet(),
		},
		packageRequests: []PackageRequest{},
		envs:            map[string]string{},
		rootRun:         []Command{},
		userRun:         []Command{},
		fileTemplates:   []FileTemplate{},
		entrypoint:      []string{},
		cmd:             []string{},
	}

	for _, opt := range opts {
		err := opt(brick)
		if err != nil {
			return nil, err
		}
	}

	return brick, nil
}

func WithCacheFolders(folders CacheFoldersPaths) BrickOption {
	return func(bi *brick) error {
		bi.cacheFolders = append(bi.cacheFolders, folders...)

		return nil
	}
}

func WithCacheFolder(folder string) BrickOption {
	return func(bi *brick) error {
		bi.cacheFolders = append(bi.cacheFolders, folder)

		return nil
	}
}

func WithKind(kind BrickKind) BrickOption {
	return func(bi *brick) error {
		kinds := append(bi.kinds.All(), kind)
		newSet := NewBrickKindsSet(kinds...)

		bi.kinds = newSet

		return nil
	}
}

func WithKinds(kinds []BrickKind) BrickOption {
	return func(bi *brick) error {
		newKinds := append(bi.kinds.All(), kinds...)
		newSet := NewBrickKindsSet(newKinds...)

		bi.kinds = newSet

		return nil
	}
}

func WithBaseImage(baseImage string) BrickOption {
	return func(bi *brick) error {
		bi.baseImage = baseImage

		return nil
	}
}

func WithPackageRequest(packageRequest PackageRequest) BrickOption {
	return func(bi *brick) error {
		bi.packageRequests = append(bi.packageRequests, packageRequest.Clone())

		return nil
	}
}

func WithPackageRequests(packageRequests []PackageRequest) BrickOption {
	return func(bi *brick) error {
		bi.packageRequests = append(bi.packageRequests, copyPackageRequests(packageRequests)...)

		return nil
	}
}

func WithEnv(k, v string) BrickOption {
	return func(bi *brick) error {
		if bi.envs == nil {
			bi.envs = make(map[string]string)
		}

		if _, ok := bi.envs[k]; ok {
			return fmt.Errorf("[brick %s] Environemnt variable %s already exists. Can't override", bi.id, k)
		}

		bi.envs[k] = v

		return nil
	}
}

func WithEnvs(envs map[string]string) BrickOption {
	return func(bi *brick) error {
		if bi.envs == nil {
			bi.envs = make(map[string]string)
		}

		for k, v := range envs {
			if _, ok := bi.envs[k]; ok {
				return fmt.Errorf("[brick %s] Environemnt variable %s already exists. Can't override", bi.id, k)
			}

			bi.envs[k] = v
		}

		return nil
	}
}

func WithRootRun(command Command) BrickOption {
	return func(bi *brick) error {
		bi.rootRun = append(bi.rootRun, command)

		return nil
	}
}

func WithRootRuns(commands []Command) BrickOption {
	return func(bi *brick) error {
		bi.rootRun = append(bi.rootRun, commands...)

		return nil
	}
}

func WithUserRun(command Command) BrickOption {
	return func(bi *brick) error {
		bi.userRun = append(bi.userRun, command)

		return nil
	}
}

func WithUserRuns(commands []Command) BrickOption {
	return func(bi *brick) error {
		bi.userRun = append(bi.userRun, commands...)

		return nil
	}
}

func WithFileTemplate(fileTemplate FileTemplate) BrickOption {
	return func(bi *brick) error {
		bi.fileTemplates = append(bi.fileTemplates, fileTemplate)

		return nil
	}
}

func WithFileTemplates(fileTemplates []FileTemplate) BrickOption {
	return func(bi *brick) error {
		bi.fileTemplates = append(bi.fileTemplates, fileTemplates...)

		return nil
	}
}

func WithEntrypoint(entrypoint []string) BrickOption {
	return func(bi *brick) error {
		if len(bi.entrypoint) > 0 {
			return fmt.Errorf("[brick %s] Entrypoint already defined. Cannot override", bi.id)
		}

		bi.entrypoint = copyStrings(entrypoint)

		return nil
	}
}

func WithCmd(cmd []string) BrickOption {
	return func(bi *brick) error {
		if len(bi.cmd) > 0 {
			return fmt.Errorf("[brick %s] CMD already defined. Cannot override", bi.id)
		}

		bi.cmd = copyStrings(cmd)

		return nil
	}
}

func WithPackageManager(manager PackageManager) BrickOption {
	return func(bi *brick) error {
		bi.packageManager = manager

		return nil
	}
}

func WithBrick(b Brick) BrickOption {
	return func(bi *brick) error {
		WithKinds(b.Kinds().All())(bi)
		WithBaseImage(b.BaseImage())(bi)
		WithPackageRequests(b.PackageRequests())(bi)
		WithEnvs(b.Envs())(bi)
		WithRootRuns(b.RootRun())(bi)
		WithUserRuns(b.UserRun())(bi)
		WithFileTemplates(b.FileTemplates())(bi)
		WithEntrypoint(b.Entrypoint())(bi)
		WithCmd(b.Cmd())(bi)
		WithCacheFolders(b.CacheFolders())(bi)

		return nil
	}
}
