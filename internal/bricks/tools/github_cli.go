package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const githubCLI = "gh"

func NewGitHubCLI(metadata map[string]string) (bricksengine.Brick, error) {
	version := metadata["version"]
	if version == "" {
		version = "2.65.0"
	}

	installCmd := `set -e && \
ARCH=$(dpkg --print-architecture) && \
TMPDIR=$(mktemp -d) && \
curl -fsSL "https://github.com/cli/cli/releases/download/v` + version + `/gh_` + version + `_linux_${ARCH}.tar.gz" -o "${TMPDIR}/gh.tar.gz" && \
tar -xzf "${TMPDIR}/gh.tar.gz" -C "${TMPDIR}" && \
install -m 0755 "${TMPDIR}/gh_` + version + `_linux_${ARCH}/bin/gh" "${MKENV_LOCAL_BIN}/gh" && \
rm -rf "${TMPDIR}"`

	brick, err := bricksengine.NewBrick(githubCLI, "GitHub CLI",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "gh install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
			},
		}),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.config/gh"),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"sh", "-c", installCmd},
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(githubCLI, NewGitHubCLI)
}
