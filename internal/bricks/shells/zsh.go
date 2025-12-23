package shells

import "github.com/0xa1bed0/mkenv/internal/bricksengine"

const zsh = "zsh"

func NewZsh(metadata map[string]string) (bricksengine.Brick, error) {
	brick, err := bricksengine.NewBrick(zsh, "ZSH shell",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithKind(bricksengine.BrickKindEntrypoint),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.zshcache"),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "install zsh",
			Packages: []bricksengine.PackageSpec{
				{Name: "zsh"},
			},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"chsh", "-s", "/bin/zsh", "${MKENV_USERNAME}"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"mkdir", "-p", "${MKENV_HOME}/.zshcache"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"ln", "-s", "${MKENV_HOME}/.zshcache/.zsh_history", "${MKENV_HOME}/.zsh_history"},
		}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "zshrc",
			FilePath: "${MKENV_HOME}/.zshrc",
			Content:  `[ -s "${MKENV_HOME}/.mkenvrc" ] && . "${MKENV_HOME}/.mkenvrc"`,
		}),
		bricksengine.WithEntrypoint([]string{"/usr/bin/zsh"}, []string{"zsh"}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(zsh, NewZsh)
}
