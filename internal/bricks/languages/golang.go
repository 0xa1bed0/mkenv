package languages

import (
	"errors"
	"fmt"
)

type golang struct {
	LangBrickBase
}

// GetDockerfilePatch implements Lang.
func (g *golang) GetDockerfilePatch() (string, error) {
	patch := `
ENV GVM_DIR=$HOME/.gvm
RUN curl -fsSL https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer | bash \
  && source $GVM_DIR/scripts/gvm \
  && gvm install go%s -B \
  && gvm use go%s --default \
  && go install golang.org/x/tools/gopls@latest \
  && go install golang.org/x/tools/cmd/goimports@latest \
  && go install golang.org/x/lint/golint@latest \
  && go install go.uber.org/mock/mockgen@latest
`

	version := g.GetVersion()
	if len(version) == 0 {
		return "", errors.New("missing golang version") // TODO: it is an error because ideally this should not happen
	}

	return fmt.Sprintf(patch, version, version), nil
}

// NewLangGolang creates a Golang brick preconfigured with a sensible default
// toolchain version.
func NewLangGolang() Lang {
	brick := &golang{}

	brick.SetName("golang")

	brick.SetVersion("1.15.3")

	brick.SetParam("targetFile", "go.mod")
	brick.SetParam("versionPrefix", "go ")

	return brick
}
