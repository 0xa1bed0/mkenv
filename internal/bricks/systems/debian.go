package systems

import "github.com/0xa1bed0/mkenv/internal/dockerfile"

const debian = "system/debian"

func NewDebian(metadata map[string]string) (dockerfile.Brick, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	base, baseExists := metadata["base"]
	if !baseExists || base == "" {
		base = "debian:bookworm-slim"
	}

	brick, err := dockerfile.NewBrick(debian, "Debian OS",
		dockerfile.WithKind(dockerfile.BrickKindSystem),
		dockerfile.WithBaseImage(base),
		dockerfile.WithPackageManager(&AptManager{}),
		// TODO: move this to platform
		dockerfile.WithCacheFolder("${MKENV_HOME}/.mkenv-pstore"),
		dockerfile.WithRootRun(dockerfile.Command{When: "build", Argv: []string{"groupadd", "--gid", "${MKENV_GID}", "${MKENV_USERNAME}"}}),
		dockerfile.WithRootRun(dockerfile.Command{When: "build", Argv: []string{"useradd", "--uid", "${MKENV_UID}", "--gid", "${MKENV_GID}", "-m", "${MKENV_USERNAME}"}}),
		dockerfile.WithRootRun(dockerfile.Command{When: "build", Argv: []string{"mkdir", "-p", "${MKENV_LOCAL_BIN}"}}),
		dockerfile.WithRootRun(dockerfile.Command{When: "build", Argv: []string{"chown", "-R", "${MKENV_USERNAME}:${MKENV_USERNAME}", "${MKENV_LOCAL_BIN}"}}),
		dockerfile.WithFileTemplate(dockerfile.FileTemplate{
			ID:       "system config",
			FilePath: "rc",
			Content: `export MKENV_LOCAL_BIN="${MKENV_LOCAL_BIN}"
export PATH="$PATH:$MKENV_LOCAL_BIN"`,
		}),
	)

	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	dockerfile.RegisterBrick(debian, NewDebian)
}
