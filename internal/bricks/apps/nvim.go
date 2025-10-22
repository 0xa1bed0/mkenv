package apps

import (
	"errors"
	"fmt"
)

type nvim struct{
	AppBrickBase
}

// GetDockerfilePatch implements App.
func (n *nvim) GetDockerfilePatch() (string, error) {
	// TODO: check the cpu architecture of the user and get arm/non arm
	patch := `
USER root
RUN curl -LO https://github.com/neovim/neovim/releases/download/%s/nvim-linux-arm64.tar.gz \
  && tar xzf nvim-linux-arm64.tar.gz \
  && mv nvim-linux-arm64 /opt/nvim \
  && ln -s /opt/nvim/bin/nvim /usr/local/bin/nvim \
  && rm nvim-linux-arm64.tar.gz
USER ${USER_NAME}

RUN mkdir -p $HOME/.config/nvim
`

	version := n.GetVersion()
	if len(version) == 0 {
		return "", errors.New("missing nvim version") // TODO: it is an error because ideally this should not happen
	}

	return fmt.Sprintf(patch, version), nil
}

func NewNvim() App {
	brick := &nvim{}

	brick.SetVersion("0.11.4")

	return brick
}
