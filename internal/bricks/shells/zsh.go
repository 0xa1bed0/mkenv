package shells

import "github.com/0xa1bed0/mkenv/internal/dockerfile"

const zsh = "shell/zsh"

func NewZsh(metadata map[string]string) (dockerfile.Brick, error) {
	brick, err := dockerfile.NewBrick(zsh, "ZSH shell",
		dockerfile.WithKind(dockerfile.BrickKindCommon),
		dockerfile.WithPackageRequest(dockerfile.PackageRequest{
			Reason: "install zsh",
			Packages: []dockerfile.PackageSpec{
				{Name: "zsh"},
			},
		}),
		dockerfile.WithRootRun(dockerfile.Command{
			When: "build",
			Argv: []string{"chsh", "-s", "/bin/zsh", "${MKENV_USERNAME}"},
		}),
		dockerfile.WithFileTemplate(dockerfile.FileTemplate{
			ID:       "zshrc",
			FilePath: "${MKENV_HOME}/.zshrc",
			Content:  `[ -s "${MKENV_HOME}/.mkenvrc" ] && . "${MKENV_HOME}/.mkenvrc"`,
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	dockerfile.RegisterBrick(zsh, NewZsh)
}
