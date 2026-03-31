package tools

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

const dockerCLI = "docker-cli"

func NewDockerCLI(map[string]string) (bricksengine.Brick, error) {
	brick, err := bricksengine.NewBrick(dockerCLI, "Docker CLI",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		// curl and ca-certificates are needed to set up Docker's official apt repo below.
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "Prerequisites for Docker CLI installation",
			Packages: []bricksengine.PackageSpec{
				{Name: "ca-certificates"},
				{Name: "curl"},
			},
		}),
		// Set up Docker's official apt repository (runs after the main package install phase).
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"sh", "-c", "install -m 0755 -d /etc/apt/keyrings && curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc && chmod a+r /etc/apt/keyrings/docker.asc"},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"sh", "-c", `echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list`},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"apt-get", "update"},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"apt-get", "install", "-y", "--no-install-recommends", "docker-ce-cli"},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"rm", "-rf", "/var/lib/apt/lists/*"},
		}),
		// docker-ce-cli does not create the docker group (only the full daemon does).
		// Create it explicitly so usermod can add the user to it.
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"groupadd", "--force", "docker"},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"usermod", "-aG", "docker", "${MKENV_USERNAME}"},
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(dockerCLI, NewDockerCLI)
}
