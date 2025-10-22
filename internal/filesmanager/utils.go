package filesmanager

import (
	"os"
	"path/filepath"
	"strings"
)

// isPlainFilename returns true if name is a single, plain filename
// (not a path, not ".", not "..", and containing no separators).
func isPlainFilename(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}

	// filepath.Base() strips directories using the OS-specific separator.
	// If it's equal to the input, there were no platform separators.
	if filepath.Base(name) != name {
		return false
	}

	// On Windows, the path separator is '\', but we should also reject '/'.
	// On Unix, the separator is '/', but reject '\' anyway—just in case.
	if strings.ContainsAny(name, `/\`) {
		return false
	}

	return true
}

// samePath compares two absolute, cleaned paths with OS semantics.
func samePath(a, b string) bool {
	if a == b {
		return true
	}
	// On Windows, case-insensitive; on POSIX, case-sensitive.
	// filepath.EvalSymlinks would resolve symlinks but is more expensive;
	// we avoid it here to keep things fast and predictable.
	if isWindows() {
		return strings.EqualFold(a, b)
	}
	return false
}

func isWindows() bool {
	// crude but effective without importing runtime
	return os.PathSeparator == '\\'
}

// toSlashClean cleans and converts to forward-slash separators.
func toSlashClean(p string) string {
	return filepath.ToSlash(filepath.Clean(p))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
