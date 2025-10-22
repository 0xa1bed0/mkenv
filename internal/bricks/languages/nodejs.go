package languages

import (
	"errors"
	"fmt"
)

type nodejs struct {
	LangBrickBase
}

// GetDockerfilePatch implements Lang.
func (n *nodejs) GetDockerfilePatch() (string, error) {
	// TODO: config nvm version
	patch := `
ENV NVM_DIR=$HOME/.nvm
RUN mkdir -p "$NVM_DIR" \
	&& curl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash \
	&& /bin/zsh -lc "source $NVM_DIR/nvm.sh && nvm install %s && nvm alias default %s"
`
	
	version := n.GetVersion()
	if len(version) == 0 {
		return "", errors.New("missing nodejs version") // TODO: it is an error because ideally this should not happen
	}

	return fmt.Sprintf(patch, version, version), nil
}

// NewLangNodejs creates a Golang brick preconfigured with a sensible default
// toolchain version.
func NewLangNodejs() Lang {
	brick := &nodejs{}

	brick.SetName("nodejs")

	brick.SetVersion("lts")

	brick.SetParam("targetFile", "package.json")
	brick.SetParam("versionPrefix", "\"node\": \"")

	return brick
}
