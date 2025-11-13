package langs

import (
	"strings"

	"github.com/0xa1bed0/mkenv/internal/dockerfile"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

const (
	golangID          = dockerfile.BrickID("lang/golang")
	golangDescription = "Golang toolchain"
)

var golangKinds = []dockerfile.BrickKind{dockerfile.BrickKindCommon}

func NewGolang(metadata map[string]string) (dockerfile.Brick, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	version, ok := metadata["version"]
	if !ok || version == "" {
		version = "go1.25.3"
	} else {
		version = "go"+strings.Replace(version, "go", "", 1)
	}

	brick, err := dockerfile.NewBrick(golangID, golangDescription,
		dockerfile.WithKinds(golangKinds),
		dockerfile.WithPackageRequest(dockerfile.PackageRequest{
			Reason: "gvm install dependencies",
			Packages: []dockerfile.PackageSpec{
				{Name: "curl"},
				{Name: "ca-certificates"},
				{Name: "git"},
				{Name: "build-essential"},
				{Name: "binutils"},
				{Name: "bsdmainutils"},
				{Name: "bison"},
			},
		}),
		dockerfile.WithCacheFolder("${MKENV_HOME}/.cache/go-build"),
		dockerfile.WithCacheFolder("${MKENV_HOME}/.cache/goimports"),
		dockerfile.WithCacheFolder("${MKENV_HOME}/.cache/gopls"),
		dockerfile.WithCacheFolder("${MKENV_HOME}/.gvm/pkgsets"),
		dockerfile.WithEnv("GVM_DIR", "${MKENV_HOME}/.gvm"),
		dockerfile.WithUserRun(dockerfile.Command{
			When: "build",
			Argv: []string{
				"/bin/bash", "-lc", `set -eo pipefail 
export GOLANG_VERSION=`+ version + ` 
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
		dockerfile.WithFileTemplate(dockerfile.FileTemplate{
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
	langDetector dockerfile.LangDetector
}

func (*golangDetector) BrickInfo() *dockerfile.BrickInfo {
	return dockerfile.NewBrickInfo(golangID, golangDescription, golangKinds)
}

func (gd *golangDetector) Scan(folderPtr filesmanager.FileManager) (dockerfile.BrickID, map[string]string, error) {
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
	dockerfile.RegisterBrick(golangID, NewGolang)
	dockerfile.RegisterDetector(func() dockerfile.BrickDetector {
		return &golangDetector{langDetector: dockerfile.NewLangDetector("go.mod", "go", "go ")}
	})
}
