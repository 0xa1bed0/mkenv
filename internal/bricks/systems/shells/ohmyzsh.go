package shells

type ohmyzsh struct {
	zsh
}

// GetDockerfilePatch implements container.Shell.
func (z *ohmyzsh) GetDockerfilePatch() (string, error) {
	z.zshrc = []string{}
	z.zshrc = append(z.zshrc, "export ZSH='$HOME/.oh-my-zsh'")
	z.zshrc = append(z.zshrc, "ZSH_THEME='robbyrussel'")
	z.zshrc = append(z.zshrc, "plugins=(git)")
	z.zshrc = append(z.zshrc, "source $ZSH/oh-my-zsh.sh")

	patch, err := z.zsh.GetDockerfilePatch()
	if err != nil {
		return "", err
	}

	patch += `
RUN export RUNZSH=no; export CHSH=no; export KEEP_ZSHRC=yes; sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
`

	return patch, nil
}

// NewShellOhMyZsh returns a shell brick that layers Oh My Zsh on top of zsh.
func NewShellOhMyZsh() Shell {
	return &ohmyzsh{ }
}
