package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const kimiCode = "kimi-code"

func NewKimiCode(metadata map[string]string) (bricksengine.Brick, error) {
	brick, err := bricksengine.NewBrick(kimiCode, "Kimi Code CLI",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "kimi-code install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
			},
		}),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.kimi-code"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.config/kimi"),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"bash", "-c", "KIMI_NO_MODIFY_PATH=1 curl -fsSL https://code.kimi.com/kimi-code/install.sh | bash"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"ln", "-sf", "${MKENV_HOME}/.kimi-code/bin/kimi", "${MKENV_LOCAL_BIN}/kimi"},
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(kimiCode, NewKimiCode)
}
