package systems

import (
	"strings"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/utils"
)

type AptManager struct{}

func (AptManager) Name() string { return "apt" }

func (m AptManager) Install(requests []bricksengine.PackageSpec) []bricksengine.Command {
	names := m.resolveNames(requests)

	// Combine apt-get update, install, and cleanup into a single command
	// to prevent Docker layer caching from using a stale package index.
	shellCmd := "apt-get update && apt-get install -y --no-install-recommends " +
		strings.Join(names, " ") +
		" && rm -rf /var/lib/apt/lists/*"

	return []bricksengine.Command{
		{When: "build", Argv: []string{"/bin/sh", "-c", shellCmd}},
	}
}

func (m AptManager) RuntimeInstall(requests []bricksengine.PackageSpec) []bricksengine.Command {
	names := m.resolveNames(requests)

	return []bricksengine.Command{
		{When: "runtime", Argv: []string{"apt-get", "update"}},
		{When: "runtime", Argv: append([]string{"apt-get", "install", "-y", "--no-install-recommends"}, names...)},
	}
}

func (AptManager) resolveNames(requests []bricksengine.PackageSpec) []string {
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
	return utils.UniqueSorted(names)
}
