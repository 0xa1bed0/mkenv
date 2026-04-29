package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const nomad = "nomad"

func NewNomad(metadata map[string]string) (bricksengine.Brick, error) {
	version := metadata["version"]
	if version == "" {
		version = "1.9.7"
	}

	installCmd := `set -e && \
ARCH=$(dpkg --print-architecture) && \
curl -fsSL "https://releases.hashicorp.com/nomad/` + version + `/nomad_` + version + `_linux_${ARCH}.zip" -o /tmp/nomad.zip && \
unzip -o /tmp/nomad.zip -d ${MKENV_LOCAL_BIN} && \
rm /tmp/nomad.zip`

	brick, err := bricksengine.NewBrick(nomad, "HashiCorp Nomad CLI",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "nomad install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
				{Name: "unzip"},
			},
		}),
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
	bricksengine.RegisterBrick(nomad, NewNomad)
}
