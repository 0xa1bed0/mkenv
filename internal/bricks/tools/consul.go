package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const consul = "consul"

func NewConsul(metadata map[string]string) (bricksengine.Brick, error) {
	version := metadata["version"]
	if version == "" {
		version = "1.21.1"
	}

	installCmd := `set -e && \
ARCH=$(dpkg --print-architecture) && \
curl -fsSL "https://releases.hashicorp.com/consul/` + version + `/consul_` + version + `_linux_${ARCH}.zip" -o /tmp/consul.zip && \
unzip -o /tmp/consul.zip -d ${MKENV_LOCAL_BIN} && \
rm /tmp/consul.zip`

	brick, err := bricksengine.NewBrick(consul, "HashiCorp Consul CLI",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "consul install dependencies",
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
	bricksengine.RegisterBrick(consul, NewConsul)
}
