package tools

import (
	"github.com/0xa1bed0/mkenv/internal/dockerfile"
)

const nvim = "tools/nvim"

func NewNvim(map[string]string) (dockerfile.Brick, error) {
	brick, err := dockerfile.NewBrick(nvim, "NeoVim",
		dockerfile.WithKind(dockerfile.BrickKindCommon),
		dockerfile.WithPackageRequest(dockerfile.PackageRequest{
			Reason: "neovim installdependencies",
			Packages: []dockerfile.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
				{Name: "tar"},
			},
		}),
		dockerfile.WithCacheFolder("${MKENV_HOME}/.local/share/nvim"),
		dockerfile.WithCacheFolder("${MKENV_HOME}/.local/state/nvim"),
		dockerfile.WithCacheFolder("${MKENV_HOME}/.cache/nvim"),
		// TODO: version and system arch
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{"curl", "-fL", "-o", "nvim.tar.gz", "https://github.com/neovim/neovim/releases/download/v0.11.4/nvim-linux-arm64.tar.gz"},
		}),
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{"mkdir", "nvim"},
		}),
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{"tar", "xzf", "nvim.tar.gz", "-C", "nvim", "--strip-components=1"},
		}),
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{"ls", "-la"},
		}),
		dockerfile.WithUserRun(dockerfile.Command{When: "build", Argv: []string{"mkdir", "-p", "${MKENV_HOME}/.opt/nvim"}}),
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{"mv", "nvim", "${MKENV_HOME}/.opt"},
		}),
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{"ln", "-s", "${MKENV_HOME}/.opt/nvim/bin/nvim", "${MKENV_LOCAL_BIN}/nvim"},
		}),
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{"rm", "nvim.tar.gz"},
		}),
		dockerfile.WithUserRun(dockerfile.Command{When: "build", Argv: []string{"mkdir", "-p", "${MKENV_HOME}/.config/nvim"}}),
		dockerfile.WithFileTemplate(dockerfile.FileTemplate{
			ID:       "nvim aliases",
			FilePath: "rc",
			Content: `# NVIM aliases start
alias vim="nvim"
alias vi="nvim"
# NVIM aliases end`,
		}),
		dockerfile.WithFileTemplate(dockerfile.FileTemplate{
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
	dockerfile.RegisterBrick(nvim, NewNvim)
}
