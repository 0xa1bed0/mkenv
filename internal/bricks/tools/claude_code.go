package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const claudeCode = "claude-code"

func NewClaudeCode(metadata map[string]string) (bricksengine.Brick, error) {
	brick, err := bricksengine.NewBrick(claudeCode, "Claude Code CLI",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "claude-code install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
			},
		}),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.claude"),
		bricksengine.WithCacheFile("${MKENV_HOME}/.claude.json"),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"bash", "-c", "curl -fsSL https://claude.ai/install.sh | bash"},
		}),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"ln", "-sf", "${MKENV_HOME}/.local/bin/claude", "${MKENV_LOCAL_BIN}/claude"},
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(claudeCode, NewClaudeCode)
}
