package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const nvim = "nvim"

func NewNvim(map[string]string) (bricksengine.Brick, error) {
	brick, err := bricksengine.NewBrick(nvim, "NeoVim",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "neovim installdependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
				{Name: "tar"},
				{Name: "build-essential"},
			},
		}),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.local/share/nvim"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.local/state/nvim"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.cache/nvim"),
		// TODO: version and system arch
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"curl", "-fL", "-o", "nvim.tar.gz", "https://github.com/neovim/neovim/releases/download/v0.11.4/nvim-linux-arm64.tar.gz"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"mkdir", "nvim"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"tar", "xzf", "nvim.tar.gz", "-C", "nvim", "--strip-components=1"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"ls", "-la"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{When: "build", Argv: []string{"mkdir", "-p", "${MKENV_HOME}/.opt/nvim"}}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"mv", "nvim", "${MKENV_HOME}/.opt"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"ln", "-s", "${MKENV_HOME}/.opt/nvim/bin/nvim", "${MKENV_LOCAL_BIN}/nvim"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"rm", "nvim.tar.gz"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{When: "build", Argv: []string{"mkdir", "-p", "${MKENV_HOME}/.config/nvim"}}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "nvim aliases",
			FilePath: "rc",
			Content: `# NVIM aliases start
alias vim="nvim"
alias vi="nvim"
# NVIM aliases end`,
		}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "set nvim as default editor",
			FilePath: "rc",
			Content: `# NVIM start
export EDITOR="nvim"
# NVIM end`,
		}),
	)

	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(nvim, NewNvim)
}
