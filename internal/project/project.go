package project

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var invalidNameChars = regexp.MustCompile(`[^a-z0-9._-]+`)


type Project struct {
	Name string
	Path string
	ImageID string
}

func ResolveProject(path string, imgTag string) *Project {
	return &Project{
		Name: projectNameFromPath(path),
		Path: path,
		ImageID: imgTag,
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

