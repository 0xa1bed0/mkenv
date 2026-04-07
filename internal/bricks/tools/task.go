package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const task = "task"

func NewTask(metadata map[string]string) (bricksengine.Brick, error) {
	version := metadata["version"]

	installCmd := "curl -fsSL https://taskfile.dev/install.sh | sh -s -- -d -b ${MKENV_LOCAL_BIN}"
	if version != "" {
		installCmd += " " + version
	}

	brick, err := bricksengine.NewBrick(task, "Task runner (taskfile.dev)",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "task install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
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
	bricksengine.RegisterBrick(task, NewTask)
}
