package systems

import (
	"github.com/0xa1bed0/mkenv/internal/dockerfile"
	"github.com/0xa1bed0/mkenv/internal/utils"
)

type AptManager struct{}

func (AptManager) Name() string { return "apt" }

func (AptManager) Install(requests []dockerfile.PackageSpec) []dockerfile.Command {
	names := []string{}
	for _, request := range requests {
		name := request.Name
		if override, ok := request.Meta["apt"]; ok && override != "" {
			name = override
		}
		if pin, ok := request.Meta["apt_pin"]; ok && pin != "" {
			name = name + "=" + pin
		}
		names = append(names, name)
	}

	names = utils.UniqueSorted(names)

	out := make([]dockerfile.Command, 3)
	out[0] = dockerfile.Command{When: "build", Argv: []string{"apt-get", "update"}}
	installCmd := []string{"apt-get", "install", "-y", "--no-install-recommends"}
	out[1] = dockerfile.Command{When: "build", Argv: append(installCmd, names...)}
	out[2] = dockerfile.Command{When: "build", Argv: []string{"rm", "-rf", "/var/lib/apt/lists/*"}}

	return out
}
