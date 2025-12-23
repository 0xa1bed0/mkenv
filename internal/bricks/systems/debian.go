package systems

import "github.com/0xa1bed0/mkenv/internal/bricksengine"

const debian = "debian"

func NewDebian(metadata map[string]string) (bricksengine.Brick, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	base, baseExists := metadata["base"]
	if !baseExists || base == "" {
		base = "debian:bookworm-slim"
	}

	brick, err := bricksengine.NewBrick(debian, "Debian OS",
		bricksengine.WithKind(bricksengine.BrickKindSystem),
		bricksengine.WithBaseImage(base),
		bricksengine.WithPackageManager(&AptManager{}),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "Convinience tools",
			Packages: []bricksengine.PackageSpec{
				{Name: "procps"},
				{Name: "fzf"},
				{Name: "ripgrep"},
				{Name: "htop"},
				{Name: "openssh-client"},
				{Name: "netcat-traditional"}, // TODO: remove it
			},
		}),
		// TODO: move this to platform
		bricksengine.WithCacheFolder("${MKENV_HOME}/.mkenv-pstore"),
		bricksengine.WithRootRun(bricksengine.Command{When: "build", Argv: []string{"groupadd", "--gid", "${MKENV_GID}", "${MKENV_USERNAME}"}}),
		bricksengine.WithRootRun(bricksengine.Command{When: "build", Argv: []string{"useradd", "--uid", "${MKENV_UID}", "--gid", "${MKENV_GID}", "-m", "${MKENV_USERNAME}"}}),
		bricksengine.WithRootRun(bricksengine.Command{When: "build", Argv: []string{"mkdir", "-p", "${MKENV_LOCAL_BIN}"}}),
		bricksengine.WithRootRun(bricksengine.Command{When: "build", Argv: []string{"chown", "-R", "${MKENV_USERNAME}:${MKENV_USERNAME}", "${MKENV_LOCAL_BIN}"}}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
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
	bricksengine.RegisterBrick(debian, NewDebian)
}
