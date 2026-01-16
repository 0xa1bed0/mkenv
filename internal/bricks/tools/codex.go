package tools

import (
	"fmt"

	"github.com/0xa1bed0/mkenv/internal/bricks/langs"
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const codex = "codex"

func NewCodex(metadata map[string]string) (bricksengine.Brick, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}

	packageSpec := "@openai/codex"
	if version, ok := metadata["version"]; ok && version != "" {
		packageSpec = fmt.Sprintf("%s@%s", packageSpec, version)
	}

	nodeMeta := map[string]string{}
	if nodeVersion, ok := metadata["node_version"]; ok && nodeVersion != "" {
		nodeMeta["version"] = nodeVersion
	}
	if len(nodeMeta) == 0 {
		nodeMeta = nil
	}

	nodeBrick, err := langs.NewNodejs(nodeMeta)
	if err != nil {
		return nil, err
	}

	brick, err := bricksengine.NewBrick(codex, "OpenAI Codex CLI",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithBrick(nodeBrick),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.npm"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.codex"),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"/bin/bash", "-lc", fmt.Sprintf(`set -eo pipefail
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
nvm use default >/dev/null
npm install -g %s
ln -sf "$(npm bin -g)/codex" "${MKENV_LOCAL_BIN}/codex"`, packageSpec)},
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(codex, NewCodex)
}
