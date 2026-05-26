package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const postgresCLI = "postgres-cli"

func NewPostgresCLI(metadata map[string]string) (bricksengine.Brick, error) {
	version := metadata["version"]
	if version == "" {
		version = "17"
	}

	keySetupCmd := `set -e && \
install -d /usr/share/postgresql-common/pgdg && \
curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc`

	repoSetupCmd := `set -e && \
echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt $(. /etc/os-release && echo "$VERSION_CODENAME")-pgdg main" > /etc/apt/sources.list.d/pgdg.list`

	brick, err := bricksengine.NewBrick(postgresCLI, "PostgreSQL CLI (psql)",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "Prerequisites for PostgreSQL apt repo",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
				{Name: "gnupg"},
			},
		}),
		bricksengine.WithCacheFile("${MKENV_HOME}/.psql_history"),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"sh", "-c", keySetupCmd},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"sh", "-c", repoSetupCmd},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"apt-get", "update"},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"apt-get", "install", "-y", "--no-install-recommends", "postgresql-client-" + version},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"sh", "-c", "rm -rf /var/lib/apt/lists/*"},
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(postgresCLI, NewPostgresCLI)
}
