package langs

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
	"github.com/0xa1bed0/mkenv/internal/versions"
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
	packageJsonDetector bricksengine.LangDetector
	npmrcDetector       bricksengine.LangDetector
}

func (*nodejsDetector) BrickInfo() *bricksengine.BrickInfo {
	return bricksengine.NewBrickInfo(nodejsID, nodejsDescription, nodejsKinds)
}

func (nd *nodejsDetector) Scan(folderPtr filesmanager.FileManager) (bricksengine.BrickID, map[string]string, error) {
	// Check package.json for version
	pkgFound, pkgMeta, err := nd.packageJsonDetector.ScanFiles(folderPtr)
	if err != nil {
		return "", nil, err
	}

	// Check .npmrc for version
	npmrcFound, npmrcMeta, err := nd.npmrcDetector.ScanFiles(folderPtr)
	if err != nil {
		return "", nil, err
	}

	// Not a nodejs project
	if !pkgFound && !npmrcFound {
		return "", nil, nil
	}

	// Combine versions from both sources
	pkgVersion := ""
	if pkgMeta != nil {
		pkgVersion = pkgMeta["version"]
	}
	npmrcVersion := ""
	if npmrcMeta != nil {
		npmrcVersion = npmrcMeta["version"]
	}

	// Determine final version
	var finalMeta map[string]string
	if pkgVersion != "" && npmrcVersion != "" {
		// Both have versions - compare and use maximum
		maxVersion, err := versions.MaxVersion([]string{pkgVersion, npmrcVersion})
		if err == nil {
			finalMeta = map[string]string{"version": maxVersion}
		} else {
			// On error, prefer package.json version
			finalMeta = map[string]string{"version": pkgVersion}
		}
	} else if pkgVersion != "" {
		finalMeta = map[string]string{"version": pkgVersion}
	} else if npmrcVersion != "" {
		finalMeta = map[string]string{"version": npmrcVersion}
	}

	return nodejsID, finalMeta, nil
}

func init() {
	bricksengine.RegisterBrick(nodejsID, NewNodejs)
	bricksengine.RegisterDetector(func() bricksengine.BrickDetector {
		return &nodejsDetector{
			packageJsonDetector: bricksengine.NewLangDetector(string(nodejsID), "package.json", "html,htm,htmlx,htmx,js,ts,jsx", `"node": "`),
			npmrcDetector:       bricksengine.NewLangDetector(string(nodejsID), ".npmrc", "html,htm,htmlx,htmx,js,ts,jsx", "node-version="),
		}
	})
}
