package utils

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	ErrNonexistentPath = errors.New("path does not exist")
	ErrPathDenied      = errors.New("path is denied by policy")
	ErrPathNotAllowed  = errors.New("path is not allowed by policy")
)

// ResolvePathStrict resolves p to an absolute, canonical path,
// following all symlinks. It fails if:
//   - the path (or any symlink in it) is broken
//   - symlink resolution fails (cycles, too deep, etc.)
func ResolvePathStrict(p string) (string, error) {
	// 1. Make absolute
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	// 2. Clean up things like "..", ".", duplicate slashes
	clean := filepath.Clean(abs)

	// 3. Resolve all symlinks
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		// includes broken symlinks, cycles, etc.
		return "", err
	}

	// 4. Final sanity: ensure the target actually exists
	if _, err := os.Stat(resolved); err != nil {
		return "", ErrNonexistentPath
	}

	return resolved, nil
}

// ResolveFolderStrict resolves path into absolute path to a folder
// If p is path to folder - returns the absolute resolved path to this folder
// If p is path to a file - returns the absolute resolved path to the folder where this file located
func ResolveFolderStrict(p string) (string, error) {
	abs, err := ResolvePathStrict(p)
	if err != nil {
		return "", err
	}

	fi, err := os.Stat(abs)
	if err != nil {
		return "", err
	}

	if !fi.IsDir() {
		return filepath.Dir(abs), nil
	}

	return abs, nil
}
