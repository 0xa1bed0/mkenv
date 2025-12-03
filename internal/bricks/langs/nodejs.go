package langs

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

const (
	nodejsID          = bricksengine.BrickID("nodejs")
	nodejsDescription = "Golang toolchain"
)

var nodejsKinds = []bricksengine.BrickKind{bricksengine.BrickKindCommon}

func NewNodejs(metadata map[string]string) (bricksengine.Brick, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	version, ok := metadata["version"]
	if !ok || version == "" {
		version = "lts/*"
	}

	brick, err := bricksengine.NewBrick(nodejsID, nodejsDescription,
		bricksengine.WithKinds(nodejsKinds),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "nvm install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "curl"},
				{Name: "ca-certificates"},
				{Name: "git"},
			},
		}),
		bricksengine.WithEnv("NVM_DIR", "${MKENV_HOME}/.nvm"),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{
				"/bin/bash", "-lc", `set -eo pipefail
export NODE_VERSION=` + version + `
curl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash
source "$NVM_DIR/nvm.sh"

guess="${NODE_VERSION#v}"
major="${guess%%.*}"

case "$NODE_VERSION" in
  lts/*)
    nvm install "$NODE_VERSION"
		nvm alias default "$NODE_VERSION"
		nvm use default
    ;;
  *)
    # try exact/partial match (e.g. 20 or 20.1)
    resolved=$(nvm ls-remote --no-colors | awk '{print $1}' | sed 's/^v//' \
      | grep -E "^${guess}(\.|$)" | tail -n1 || true)

    # if nothing, fallback to same major
    [ -z "$resolved" ] && resolved=$(nvm ls-remote --no-colors | awk '{print $1}' | sed 's/^v//' \
      | grep -E "^${major}\." | tail -n1)

    [ -z "$resolved" ] && exit 1

    NODE_VERSION="$resolved"
    nvm install "v$NODE_VERSION"
		nvm alias default "v$NODE_VERSION"
		nvm use default
    ;;
esac
	`,
			},
		}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "lang/nodejs",
			FilePath: "rc",
			Content: `# Nodejs version manager start 
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
[ -s "$NVM_DIR/bash_completion" ] && . "$NVM_DIR/bash_completion"
# Nodejs version manager end`,
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

type nodejsDetector struct {
	langDetector bricksengine.LangDetector
}

func (*nodejsDetector) BrickInfo() *bricksengine.BrickInfo {
	return bricksengine.NewBrickInfo(nodejsID, nodejsDescription, nodejsKinds)
}

func (gd *nodejsDetector) Scan(folderPtr filesmanager.FileManager) (bricksengine.BrickID, map[string]string, error) {
	found, brickMeta, err := gd.langDetector.ScanFiles(folderPtr)
	if err != nil {
		return "", nil, err
	}
	if found {
		return nodejsID, brickMeta, nil
	}
	return "", nil, nil
}

func init() {
	bricksengine.RegisterBrick(nodejsID, NewNodejs)
	bricksengine.RegisterDetector(func() bricksengine.BrickDetector {
		return &nodejsDetector{langDetector: bricksengine.NewLangDetector(string(nodejsID), "package.json", "js,ts,jsx", `"node": "`)}
	})
}
