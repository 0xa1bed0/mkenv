package shells

import (
	"fmt"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/dockerimage"
)

type zsh struct {
	dockerimage.BrickBase
	zshrc  []string
}

// GetDockerfilePatch implements container.Shell.
func (z *zsh) GetDockerfilePatch() (string, error) {
	patch := `USER root 
RUN apt-get install -y zsh
RUN chsh -s $(which zsh) ${USER_NAME}
USER ${USER_NAME}

`

	// TODO: put default lines before or after (i think after because we dont want user override them
	lines := []string{}
	for _, line := range z.zshrc {
		lines = append(lines, fmt.Sprintf("echo '%s' >> $HOME/.zshrc", line))
	}

	if len(lines) > 0 {
		patch += "RUN " + strings.Join(lines, " && ")
	}

	return patch, nil
}

// SetRCConfigs implements container.Shell.
func (z *zsh) SetRCConfigs(configs []string) error {
	z.zshrc = configs
	return nil
}

// NewShellZSH returns a shell brick that wires up a vanilla zsh environment.
func NewShellZSH() Shell {
	return &zsh{}
}
