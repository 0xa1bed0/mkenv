package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const pulumi = "pulumi"

func NewPulumi(map[string]string) (bricksengine.Brick, error) {
	brick, err := bricksengine.NewBrick(pulumi, "Infrastructure as Code (Pulumi CLI)",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "pulumi cli dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
			},
		}),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.pulumi"),
		// Install Pulumi using official installation script
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"sh", "-c", "curl -fsSL https://get.pulumi.com | sh"},
		}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "pulumi",
			FilePath: "rc",
			Content:  `export PATH="$PATH:/home/dev/.pulumi/bin"`,
		}),
	)

	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(pulumi, NewPulumi)
}
