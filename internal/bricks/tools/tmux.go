package tools

import (
	"github.com/0xa1bed0/mkenv/internal/dockerfile"
)

const tmux = "tools/tmux"

func NewTmux(map[string]string) (dockerfile.Brick, error) {
	brick, err := dockerfile.NewBrick(tmux, "Terminal multiplexer",
		dockerfile.WithKind(dockerfile.BrickKindCommon),
		dockerfile.WithKind(dockerfile.BrickKindEntrypoint),
		dockerfile.WithPackageRequest(dockerfile.PackageRequest{
			Reason: "terminal multiplexer",
			Packages: []dockerfile.PackageSpec{
				{Name: "tmux"},
			},
		}),
		dockerfile.WithEntrypoint([]string{"/usr/bin/tmux", "-u"}),
		// TODO: make it user config (extraSteps???) - consider security checks -- or make brick with tmux plugins ???
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{"mkdir", "-p", "${MKENV_HOME}/.config/tmux/plugins/catppuccin"},
		}),
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{"git", "clone", "-b", "v2.1.3", "https://github.com/catppuccin/tmux.git", "${MKENV_HOME}/.config/tmux/plugins/catppuccin/tmux"},
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	dockerfile.RegisterBrick(tmux, NewTmux)
}
