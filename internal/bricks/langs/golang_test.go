package langs

import (
	"testing"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

func createGolangDetector() *golangDetector {
	return &golangDetector{
		langDetector: bricksengine.NewLangDetector(string(golangID), "go.mod", "go", "go ", bricksengine.WithVersionSemantics(bricksengine.VersionSemanticsMinimum)),
	}
}

func TestGolangDetector_RootGoModPriority(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"go.mod":     "module example.com/root\n\ngo 1.22\n",
		"main.go":    "package main\n",
		"sub/go.mod": "module example.com/sub\n\ngo 1.25.3\n",
		"sub/sub.go": "package sub\n",
	})

	detector := createGolangDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != golangID {
		t.Errorf("expected brickID=%s, got %s", golangID, brickID)
	}
	// Root go.mod has 1.22 — should use it (priority), not scan sub/go.mod
	if meta["version"] != "1.22.0" {
		t.Errorf("expected version=1.22.0 (root go.mod priority), got %s", meta["version"])
	}
}

func TestGolangDetector_NoRootGoMod_FallsThrough(t *testing.T) {
	t.Parallel()

	// No root go.mod, but sub-project has one — should fall through to langDetector
	fm := newStubFileManager(map[string]string{
		"main.go":    "package main\n",
		"sub/go.mod": "module example.com/sub\n\ngo 1.23\n",
		"sub/sub.go": "package sub\n",
	})

	detector := createGolangDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != golangID {
		t.Errorf("expected brickID=%s, got %s", golangID, brickID)
	}
	if meta["version"] != "1.23.0" {
		t.Errorf("expected version=1.23.0 (from sub go.mod fallback), got %s", meta["version"])
	}
}

func TestGolangDetector_RootGoModWithoutVersion_FallsThrough(t *testing.T) {
	t.Parallel()

	// Root go.mod exists but has no go directive — should fall through
	fm := newStubFileManager(map[string]string{
		"go.mod":     "module example.com/root\n\nrequire (\n\tgithub.com/foo v1.0.0\n)\n",
		"main.go":    "package main\n",
		"sub/go.mod": "module example.com/sub\n\ngo 1.21\n",
		"sub/sub.go": "package sub\n",
	})

	detector := createGolangDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != golangID {
		t.Errorf("expected brickID=%s, got %s", golangID, brickID)
	}
	if meta["version"] != "1.21.0" {
		t.Errorf("expected version=1.21.0 (fallback from sub go.mod), got %s", meta["version"])
	}
}

func TestGolangDetector_NoGoModAtAll(t *testing.T) {
	t.Parallel()

	// Only .go files, no go.mod — should detect golang but no version
	fm := newStubFileManager(map[string]string{
		"main.go": "package main\n",
	})

	detector := createGolangDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != golangID {
		t.Errorf("expected brickID=%s, got %s", golangID, brickID)
	}
	if meta != nil && meta["version"] != "" {
		t.Errorf("expected no version when no go.mod, got %s", meta["version"])
	}
}

func TestGolangDetector_NoGoFiles(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"index.js": "console.log('hello');\n",
	})

	detector := createGolangDetector()
	brickID, _, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != "" {
		t.Errorf("expected empty brickID for non-golang project, got %s", brickID)
	}
}
