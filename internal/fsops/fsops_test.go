// Tests in this file cover the default filesystem operations wiring.
package fsops

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultOpsPathMethods(t *testing.T) {
	t.Parallel()

	ops := DefaultOps()

	abs, err := ops.Path.Abs(".")
	if err != nil {
		t.Fatalf("Abs failed: %v", err)
	}
	if !ops.Path.IsAbs(abs) {
		t.Fatalf("Abs returned non-absolute path: %q", abs)
	}

	rel, err := ops.Path.Rel(abs, filepath.Join(abs, "mocks"))
	if err != nil {
		t.Fatalf("Rel failed: %v", err)
	}
	if rel != "mocks" {
		t.Fatalf("Rel returned %q, want %q", rel, "mocks")
	}

	joined := ops.Path.Join("mocks", "fsops.go")
	if !strings.HasSuffix(joined, filepath.Join("mocks", "fsops.go")) {
		t.Fatalf("Join result %q missing expected segment", joined)
	}

	clean := ops.Path.Clean(filepath.Join("mocks", "..", "fsops.go"))
	if clean != "fsops.go" {
		t.Fatalf("Clean returned %q, want %q", clean, "fsops.go")
	}
}

func TestStdOSOpsStat(t *testing.T) {
	t.Parallel()

	fi, err := stdOSOps{}.Stat("fsops.go")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if fi.Name() != "fsops.go" {
		t.Fatalf("Stat returned file %q, want %q", fi.Name(), "fsops.go")
	}
}

func TestStdDirWalkerVisitsEntries(t *testing.T) {
	t.Parallel()

	root := "."
	walker := stdDirWalker{}
	visited := map[string]struct{}{}
	err := walker.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		visited[d.Name()] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	want := []string{"fsops.go", "fsops_test.go", "mocks"}
	for _, name := range want {
		if _, ok := visited[name]; !ok {
			t.Fatalf("WalkDir did not visit %q; visited=%v", name, visited)
		}
	}
}

func TestDefaultOpsPlatformIndependence(t *testing.T) {
	t.Parallel()

	ops := DefaultOps()
	path := "C:\\Temp\\file.txt"
	if runtime.GOOS != "windows" {
		path = "/tmp/file.txt"
	}
	if ops.Path.Clean(path) == "" {
		t.Fatal("Clean returned empty path")
	}
}
