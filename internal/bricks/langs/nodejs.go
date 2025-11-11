package langs

import (
	"github.com/0xa1bed0/mkenv/internal/dockerfile"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

const (
	nodejsID          = dockerfile.BrickID("lang/nodejs")
	nodejsDescription = "Golang toolchain"
)

var nodejsKinds = []dockerfile.BrickKind{dockerfile.BrickKindCommon}

func NewNodejs(metadata map[string]string) (dockerfile.Brick, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	version, ok := metadata["version"]
	if !ok || version == "" {
		version = "lts/*"
	}

	brick, err := dockerfile.NewBrick(nodejsID, nodejsDescription,
		dockerfile.WithKinds(nodejsKinds),
		dockerfile.WithPackageRequest(dockerfile.PackageRequest{
			Reason: "nvm install dependencies",
			Packages: []dockerfile.PackageSpec{
				{Name: "curl"},
				{Name: "ca-certificates"},
				{Name: "git"},
			},
		}),
		dockerfile.WithEnv("NVM_DIR", "${MKENV_HOME}/.nvm"),
		dockerfile.WithUserRun(dockerfile.Command{
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
    ;;
esac
nvm alias default "v$NODE_VERSION"
nvm use default
	`,
			},
		}),
		dockerfile.WithFileTemplate(dockerfile.FileTemplate{
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
	langDetector dockerfile.LangDetector
}

func (*nodejsDetector) BrickInfo() *dockerfile.BrickInfo {
	return dockerfile.NewBrickInfo(nodejsID, nodejsDescription, nodejsKinds)
}

func (gd *nodejsDetector) Scan(folderPtr filesmanager.FileManager) (dockerfile.BrickID, map[string]string, error) {
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
	dockerfile.RegisterBrick(nodejsID, NewNodejs)
	dockerfile.RegisterDetector(func() dockerfile.BrickDetector {
		return &nodejsDetector{langDetector: dockerfile.NewLangDetector("package.json", "js,ts,jsx", `"node": "`)}
	})
}
