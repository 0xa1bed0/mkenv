package langs

import (
	"strings"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

const (
	golangID          = bricksengine.BrickID("golang")
	golangDescription = "Golang toolchain"
)

var golangKinds = []bricksengine.BrickKind{bricksengine.BrickKindCommon}

func NewGolang(metadata map[string]string) (bricksengine.Brick, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	version, ok := metadata["version"]
	if !ok || version == "" {
		version = "go1.25.3"
	} else {
		version = "go" + strings.Replace(version, "go", "", 1)
	}

	brick, err := bricksengine.NewBrick(golangID, golangDescription,
		bricksengine.WithKinds(golangKinds),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "gvm install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "curl"},
				{Name: "ca-certificates"},
				{Name: "git"},
				{Name: "build-essential"},
				{Name: "binutils"},
				{Name: "bsdmainutils"},
				{Name: "bison"},
			},
		}),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.cache/go-build"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.cache/goimports"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.cache/gopls"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.gvm/pkgsets"),
		bricksengine.WithEnv("GVM_DIR", "${MKENV_HOME}/.gvm"),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{
				"/bin/bash", "-lc", `set -eo pipefail 
export GOLANG_VERSION=` + version + ` 
curl -fsSL https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer | bash || true
if [ ! -s "$GVM_DIR/scripts/gvm" ]; then
  echo "gvm install failed"; exit 1
fi
# gvm scripts sometimes assume looser shell settings,
# so don't let a weird non-zero blow up the build here.
set +e
source "$GVM_DIR/scripts/gvm"
rc=$?
set -e
if [ $rc -ne 0 ]; then
  echo 'failed to source gvm'; exit $rc
fi
gvm install $GOLANG_VERSION -B 
gvm use $GOLANG_VERSION --default 
go install golang.org/x/tools/gopls@latest 
go install golang.org/x/tools/cmd/goimports@latest 
go install golang.org/x/lint/golint@latest 
go install go.uber.org/mock/mockgen@latest`,
			},
		}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "lang/golang",
			FilePath: "rc",
			Content: `# Golang version manager start
[ -s "$GVM_DIR/scripts/gvm" ] && . "$GVM_DIR/scripts/gvm"
# Golang version manager end`,
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

type golangDetector struct {
	langDetector bricksengine.LangDetector
}

func (*golangDetector) BrickInfo() *bricksengine.BrickInfo {
	return bricksengine.NewBrickInfo(golangID, golangDescription, golangKinds)
}

func (gd *golangDetector) Scan(folderPtr filesmanager.FileManager) (bricksengine.BrickID, map[string]string, error) {
	found, brickMeta, err := gd.langDetector.ScanFiles(folderPtr)
	if err != nil {
		return "", nil, err
	}
	if found {
		return golangID, brickMeta, nil
	}
	return "", nil, nil
}

func init() {
	bricksengine.RegisterBrick(golangID, NewGolang)
	bricksengine.RegisterDetector(func() bricksengine.BrickDetector {
		return &golangDetector{langDetector: bricksengine.NewLangDetector(string(golangID), "go.mod", "go", "go ", bricksengine.WithVersionSemantics(bricksengine.VersionSemanticsMinimum))}
	})
}
