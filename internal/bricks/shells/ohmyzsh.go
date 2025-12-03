package shells

import "github.com/0xa1bed0/mkenv/internal/bricksengine"

const ohmyzsh = "ohmyzsh"

func NewOhMyZsh(_ map[string]string) (bricksengine.Brick, error) {
	zsh, err := NewZsh(nil)

	if err != nil {
		return nil, err
	}

	brick, err := bricksengine.NewBrick(ohmyzsh, "OhMyZsh plugin for ZSH. Installs ZSH",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithBrick(zsh),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "OhMyZsh install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "git"},
			},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"/bin/bash", "-lc", `
export RUNZSH=no
export CHSH=no 
export KEEP_ZSHRC=yes
export HOME=/tmp
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
mv /tmp/.oh-my-zsh ${MKENV_HOME}/.oh-my-zsh`},
		}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
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

func init() {
	bricksengine.RegisterBrick(ohmyzsh, NewOhMyZsh)
}
