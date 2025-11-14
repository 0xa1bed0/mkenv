package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var invalidNameChars = regexp.MustCompile(`[^a-z0-9._-]+`)

type Project struct {
	Name    string
	Path    string
	ImageID string
}

func ResolveProject(givenPath string) *Project {
	path := resolveProjectPath(givenPath)

	return &Project{
		Name:    projectNameFromPath(path),
		Path:    path,
		ImageID: "", // resolve from projects.json - this can be also used to list projects
	}
}

// projectNameFromPath: encodes (almost) the full path into a Docker-safe name.
func projectNameFromPath(input string) string {
	if input == "" {
		input = "."
	}

	abs, _ := filepath.Abs(input)
	if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
		abs = filepath.Dir(abs)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}

	home, _ := os.UserHomeDir()
	asSlash := filepath.ToSlash(abs)
	homeSlash := filepath.ToSlash(home)

	if homeSlash != "" && strings.HasPrefix(asSlash, homeSlash) {
		asSlash = strings.Replace(asSlash, homeSlash, "home", 1)
	}
	asSlash = strings.TrimPrefix(asSlash, "/")

	if runtime.GOOS == "windows" {
		if len(asSlash) >= 2 && asSlash[1] == ':' {
			asSlash = asSlash[2:]
			asSlash = strings.TrimPrefix(asSlash, "/")
		}
	}

	name := strings.ToLower(strings.ReplaceAll(asSlash, "/", "-"))
	name = invalidNameChars.ReplaceAllString(name, "_")
	name = strings.TrimLeft(name, ".-")
	if name == "" {
		name = "project"
	}

	// Don't constrain project length here; we handle final length in ContainerName.
	return name
}

func resolveProjectPath(input string) string {
	// Normalize to absolute path
	abs, err := filepath.Abs(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid path: %v\n", err)
		os.Exit(1)
	}
	pathExists, err := pathExists(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !pathExists {
		fmt.Fprintf(os.Stderr, "Error: path %s does not exists\n", abs)
		os.Exit(1)
	}
	return abs
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil // exists (file or dir)
	}
	if os.IsNotExist(err) {
		return false, nil // does not exist
	}
	return false, err // some other error (e.g. permission denied)
}
