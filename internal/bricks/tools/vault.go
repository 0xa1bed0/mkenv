package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const vault = "vault"

func NewVault(metadata map[string]string) (bricksengine.Brick, error) {
	version := metadata["version"]
	if version == "" {
		version = "1.19.0"
	}

	installCmd := `set -e && \
ARCH=$(dpkg --print-architecture) && \
curl -fsSL "https://releases.hashicorp.com/vault/` + version + `/vault_` + version + `_linux_${ARCH}.zip" -o /tmp/vault.zip && \
unzip -o /tmp/vault.zip -d ${MKENV_LOCAL_BIN} && \
rm /tmp/vault.zip`

	brick, err := bricksengine.NewBrick(vault, "HashiCorp Vault CLI",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "vault install dependencies",
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
	bricksengine.RegisterBrick(vault, NewVault)
}
