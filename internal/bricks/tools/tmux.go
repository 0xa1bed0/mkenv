package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const tmux = "tmux"

func NewTmux(map[string]string) (bricksengine.Brick, error) {
	brick, err := bricksengine.NewBrick(tmux, "Terminal multiplexer",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithKind(bricksengine.BrickKindEntrypoint),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "terminal multiplexer",
			Packages: []bricksengine.PackageSpec{
				{Name: "tmux"},
			},
		}),
		bricksengine.WithEntrypoint([]string{"/usr/bin/tmux", "-u"}, []string{"tmux", "a"}),
		// TODO: make it user config (extraSteps???) - consider security checks -- or make brick with tmux plugins ???
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"mkdir", "-p", "${MKENV_HOME}/.config/tmux/plugins/catppuccin"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
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
	bricksengine.RegisterBrick(tmux, NewTmux)
}
