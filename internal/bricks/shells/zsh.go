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
		// Register mkenv shell completion. Deferred via precmd so it runs after
		// the rest of zsh init (e.g. oh-my-zsh's compinit) — compdef needs
		// compinit to have been called first. Falls back to running compinit
		// ourselves for plain-zsh setups without oh-my-zsh.
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "mkenvcompletion",
			FilePath: "${MKENV_HOME}/.zshrc",
			Content: `if command -v mkenv >/dev/null 2>&1; then
    _mkenv_install_completion() {
        autoload -Uz add-zsh-hook
        add-zsh-hook -d precmd _mkenv_install_completion
        if (( ! ${+functions[compdef]} )); then
            autoload -Uz compinit
            compinit -u
        fi
        source <(mkenv completion zsh 2>/dev/null) 2>/dev/null
        unfunction _mkenv_install_completion 2>/dev/null
    }
    autoload -Uz add-zsh-hook
    add-zsh-hook precmd _mkenv_install_completion
fi`,
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
