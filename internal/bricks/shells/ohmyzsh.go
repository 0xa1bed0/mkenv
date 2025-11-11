package shells

import "github.com/0xa1bed0/mkenv/internal/dockerfile"

const ohmyzsh = "shell/ohmyzsh"

func NewOhMyZsh(_ map[string]string) (dockerfile.Brick, error) {
	zsh, err := NewZsh(nil)

	if err != nil {
		return nil, err
	}

	brick, err := dockerfile.NewBrick(ohmyzsh, "OhMyZsh plugin for ZSH. Installs ZSH",
		dockerfile.WithKind(dockerfile.BrickKindCommon),
		dockerfile.WithBrick(zsh),
		dockerfile.WithPackageRequest(dockerfile.PackageRequest{
			Reason: "OhMyZsh install dependencies",
			Packages: []dockerfile.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "git"},
			},
		}),
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{"/bin/bash", "-lc", `
export RUNZSH=no
export CHSH=no 
export KEEP_ZSHRC=yes
export HOME=/tmp
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
mv /tmp/.oh-my-zsh ${MKENV_HOME}/.oh-my-zsh`},
		}),
		dockerfile.WithFileTemplate(dockerfile.FileTemplate{
			ID:       "ohmyzshrc",
			FilePath: "${MKENV_HOME}/.zshrc",
			Content: `# OhMyZsh config start 
export ZSH="$HOME"/.oh-my-zsh
ZSH_THEME="robbyrussell"
plugins=(git)
source $ZSH/oh-my-zsh.sh
# OhMyZsh config end`,
		}),
	)

	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init () {
	dockerfile.RegisterBrick(ohmyzsh, NewOhMyZsh)
}
