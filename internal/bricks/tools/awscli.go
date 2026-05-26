package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const awscli = "awscli"

func NewAWSCLI(metadata map[string]string) (bricksengine.Brick, error) {
	version := metadata["version"]

	zipName := "awscli-exe-linux-${AWS_ARCH}.zip"
	if version != "" {
		zipName = "awscli-exe-linux-${AWS_ARCH}-" + version + ".zip"
	}

	smpVersion := metadata["session-manager-plugin-version"]
	if smpVersion == "" {
		smpVersion = "latest"
	}

	installCmd := `set -e && \
DPKG_ARCH=$(dpkg --print-architecture) && \
case "${DPKG_ARCH}" in \
  amd64) AWS_ARCH=x86_64 ;; \
  arm64) AWS_ARCH=aarch64 ;; \
  *) echo "unsupported architecture: ${DPKG_ARCH}" >&2; exit 1 ;; \
esac && \
TMPDIR=$(mktemp -d) && \
curl -fsSL "https://awscli.amazonaws.com/` + zipName + `" -o "${TMPDIR}/awscliv2.zip" && \
unzip -q "${TMPDIR}/awscliv2.zip" -d "${TMPDIR}" && \
"${TMPDIR}/aws/install" -i "${MKENV_HOME}/.opt/aws-cli" -b "${MKENV_LOCAL_BIN}" --update && \
rm -rf "${TMPDIR}"`

	smpInstallCmd := `set -e && \
DPKG_ARCH=$(dpkg --print-architecture) && \
case "${DPKG_ARCH}" in \
  amd64) SMP_ARCH=ubuntu_64bit ;; \
  arm64) SMP_ARCH=ubuntu_arm64 ;; \
  *) echo "unsupported architecture: ${DPKG_ARCH}" >&2; exit 1 ;; \
esac && \
TMPDIR=$(mktemp -d) && \
curl -fsSL "https://s3.amazonaws.com/session-manager-downloads/plugin/` + smpVersion + `/${SMP_ARCH}/session-manager-plugin.deb" -o "${TMPDIR}/session-manager-plugin.deb" && \
dpkg -i "${TMPDIR}/session-manager-plugin.deb" && \
rm -rf "${TMPDIR}"`

	brick, err := bricksengine.NewBrick(awscli, "AWS CLI v2 with Session Manager plugin",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "awscli install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
				{Name: "unzip"},
				{Name: "groff"},
				{Name: "less"},
			},
		}),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.aws"),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"sh", "-c", installCmd},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"sh", "-c", smpInstallCmd},
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(awscli, NewAWSCLI)
}
