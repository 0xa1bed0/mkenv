package filesmanager

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsPlainFilename(t *testing.T) {
	t.Parallel()
	tests := map[string]bool{
		"":          false,
		".":         false,
		"..":        false,
		"go.mod":    true,
		"nested/go": false,
		`other\go`:  false,
		"space ok":  true,
	}

	for input, want := range tests {
		input, want := input, want
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if got := isPlainFilename(input); got != want {
				t.Fatalf("isPlainFilename(%q) = %v, want %v", input, got, want)
			}
		})
	}
}

func TestSamePath(t *testing.T) {
	t.Parallel()

	if !samePath("/tmp/a", "/tmp/a") {
		t.Fatal("samePath should treat identical paths as equal")
	}

	if samePath("/tmp/a", "/tmp/b") {
		t.Fatal("samePath should detect different paths")
	}

	if runtime.GOOS == "windows" {
		if !samePath(`C:\Users\Alice`, `c:\users\alice`) {
			t.Fatal("samePath should be case-insensitive on Windows")
		}
	} else {
		if samePath("/tmp/Foo", "/tmp/foo") {
			t.Fatal("samePath should be case-sensitive on non-Windows systems")
		}
	}
}

func TestToSlashClean(t *testing.T) {
	t.Parallel()

	path := filepath.Join("foo", "..", "bar", "baz")
	got := toSlashClean(path)
	if got != "bar/baz" {
		t.Fatalf("toSlashClean(%q) = %q, want %q", path, got, "bar/baz")
	}
}
