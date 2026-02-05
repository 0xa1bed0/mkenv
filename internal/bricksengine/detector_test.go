package bricksengine

import (
	"bufio"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

// stubFileManager is an in-memory implementation of FileManager for testing.
type stubFileManager struct {
	// files maps relative file paths to their contents
	files map[string]string
}

func newStubFileManager(files map[string]string) *stubFileManager {
	return &stubFileManager{files: files}
}

func (s *stubFileManager) FindFile(filename string, ignorePaths []string) ([]string, error) {
	var results []string
	for path := range s.files {
		if filepath.Base(path) == filename {
			// Check if path should be ignored
			ignored := false
			for _, ignore := range ignorePaths {
				if strings.Contains(path, ignore+"/") || strings.HasPrefix(path, ignore+"/") {
					ignored = true
					break
				}
			}
			if !ignored {
				results = append(results, path)
			}
		}
	}
	return results, nil
}

func (s *stubFileManager) HasFilesWithExtensions(extsCSV string, ignorePaths []string) (bool, error) {
	exts := strings.Split(extsCSV, ",")
	extSet := make(map[string]struct{})
	for _, ext := range exts {
		ext = strings.TrimSpace(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extSet[strings.ToLower(ext)] = struct{}{}
	}

	for path := range s.files {
		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := extSet[ext]; ok {
			// Check if path should be ignored
			ignored := false
			for _, ignore := range ignorePaths {
				if strings.Contains(path, ignore+"/") || strings.HasPrefix(path, ignore+"/") {
					ignored = true
					break
				}
			}
			if !ignored {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *stubFileManager) GetFileScanner(path string, bufSize uint8) (filesmanager.FileScanner, error) {
	content, ok := s.files[path]
	if !ok {
		return nil, errors.New("file not found: " + path)
	}
	return newStubFileScanner(content), nil
}

// stubFileScanner is an in-memory implementation of FileScanner for testing.
type stubFileScanner struct {
	reader   *bufio.Reader
	hasStash bool
	stash    byte
}

func newStubFileScanner(content string) *stubFileScanner {
	return &stubFileScanner{
		reader: bufio.NewReader(strings.NewReader(content)),
	}
}

func (s *stubFileScanner) Find(prefix []byte) error {
	if len(prefix) == 0 {
		return nil
	}

	// Simple KMP implementation
	lps := make([]int, len(prefix))
	length := 0
	for i := 1; i < len(prefix); i++ {
		for length > 0 && prefix[i] != prefix[length] {
			length = lps[length-1]
		}
		if prefix[i] == prefix[length] {
			length++
			lps[i] = length
		}
	}

	j := 0
	for {
		b, err := s.readByte()
		if err != nil {
			if err == io.EOF {
				return errors.New("prefix not found")
			}
			return err
		}

		for j > 0 && b != prefix[j] {
			j = lps[j-1]
		}
		if b == prefix[j] {
			j++
			if j == len(prefix) {
				return nil
			}
		}
	}
}

func (s *stubFileScanner) ReadWhile(max filesmanager.KiB, accept func(b byte) bool) ([]byte, error) {
	if max <= 0 {
		return nil, errors.New("max must be > 0")
	}

	maxBytes := int(max) * 1024
	out := make([]byte, 0, 64)

	for {
		b, err := s.readByte()
		if err != nil {
			if err == io.EOF {
				return out, nil
			}
			return nil, err
		}

		if !accept(b) {
			s.unreadByte(b)
			return out, nil
		}

		if len(out) == maxBytes {
			return out, nil
		}
		out = append(out, b)
	}
}

func (s *stubFileScanner) Close() error {
	return nil
}

func (s *stubFileScanner) readByte() (byte, error) {
	if s.hasStash {
		s.hasStash = false
		return s.stash, nil
	}
	return s.reader.ReadByte()
}

func (s *stubFileScanner) unreadByte(b byte) {
	s.hasStash = true
	s.stash = b
}

func TestLangDetector_Golang_SingleGoModWithVersion(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"go.mod":  "module example.com/test\n\ngo 1.21\n",
		"main.go": "package main\n",
	})

	detector := NewLangDetector("golang", "go.mod", "go", "go ")
	found, meta, err := detector.ScanFiles(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected found=true")
	}
	// MaxVersionFromConstraints normalizes to semver format (adds .0)
	if meta["version"] != "1.21.0" {
		t.Errorf("expected version=1.21.0, got %s", meta["version"])
	}
}

func TestLangDetector_Golang_MultipleGoMod_FirstWithoutVersion(t *testing.T) {
	t.Parallel()

	// This test verifies the bug fix: if the first go.mod lacks a version,
	// the detector should continue checking other files instead of stopping.
	fm := newStubFileManager(map[string]string{
		"go.mod":           "module example.com/root\n\nrequire (\n\tgithub.com/foo v1.0.0\n)\n",
		"subpkg/go.mod":    "module example.com/subpkg\n\ngo 1.22\n",
		"main.go":          "package main\n",
		"subpkg/subpkg.go": "package subpkg\n",
	})

	detector := NewLangDetector("golang", "go.mod", "go", "go ")
	found, meta, err := detector.ScanFiles(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected found=true")
	}
	// MaxVersionFromConstraints normalizes to semver format
	if meta["version"] != "1.22.0" {
		t.Errorf("expected version=1.22.0 (from subpkg), got %s", meta["version"])
	}
}

func TestLangDetector_Golang_MultipleGoModWithConflicts(t *testing.T) {
	t.Parallel()

	// When multiple go.mod files specify different versions,
	// MaxVersion should pick the highest.
	fm := newStubFileManager(map[string]string{
		"go.mod":        "module example.com/root\n\ngo 1.20\n",
		"subpkg/go.mod": "module example.com/subpkg\n\ngo 1.22\n",
		"main.go":       "package main\n",
	})

	detector := NewLangDetector("golang", "go.mod", "go", "go ")
	found, meta, err := detector.ScanFiles(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected found=true")
	}
	// MaxVersionFromConstraints normalizes to semver format
	if meta["version"] != "1.22.0" {
		t.Errorf("expected version=1.22.0 (max of 1.20 and 1.22), got %s", meta["version"])
	}
}

func TestLangDetector_Golang_NoGoModButGoFilesExist(t *testing.T) {
	t.Parallel()

	// If there are .go files but no go.mod, detector should find golang
	// but have no version metadata (since we can't determine version).
	fm := newStubFileManager(map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
		"util.go": "package main\n",
	})

	detector := NewLangDetector("golang", "go.mod", "go", "go ")
	found, meta, err := detector.ScanFiles(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected found=true (go files exist)")
	}
	// No go.mod means no version detection - meta should be nil
	if meta != nil {
		t.Errorf("expected meta=nil when no go.mod, got %v", meta)
	}
}

func TestLangDetector_Nodejs_PackageJsonWithNodeEngine(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"package.json": `{
  "name": "test-project",
  "engines": {
    "node": ">=18.0.0"
  }
}`,
		"index.js": "console.log('hello');\n",
	})

	detector := NewLangDetector("nodejs", "package.json", "js,ts,jsx,tsx", `"node": "`)
	found, meta, err := detector.ScanFiles(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected found=true")
	}
	// MaxVersionFromConstraints resolves constraint ">=18.0.0" to "18.0.0"
	if meta["version"] != "18.0.0" {
		t.Errorf("expected version=18.0.0, got %s", meta["version"])
	}
}

func TestLangDetector_Nodejs_MultiplePackageJson_FirstWithoutEngine(t *testing.T) {
	t.Parallel()

	// Similar to go.mod test - first package.json lacks engine, second has it.
	fm := newStubFileManager(map[string]string{
		"package.json": `{
  "name": "root-project",
  "dependencies": {}
}`,
		"packages/sub/package.json": `{
  "name": "sub-package",
  "engines": {
    "node": ">=20"
  }
}`,
		"index.js":              "console.log('root');\n",
		"packages/sub/index.js": "console.log('sub');\n",
	})

	detector := NewLangDetector("nodejs", "package.json", "js,ts,jsx,tsx", `"node": "`)
	found, meta, err := detector.ScanFiles(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected found=true")
	}
	// MaxVersionFromConstraints resolves constraint ">=20" to "20.0.0"
	if meta["version"] != "20.0.0" {
		t.Errorf("expected version=20.0.0 (from sub-package), got %s", meta["version"])
	}
}

func TestLangDetector_Nodejs_OnlyJSFilesNoPackageJson(t *testing.T) {
	t.Parallel()

	// If there are JS files but no package.json, detector should find nodejs
	// but have no version metadata.
	fm := newStubFileManager(map[string]string{
		"index.js": "console.log('hello');\n",
		"lib.js":   "module.exports = {};\n",
	})

	detector := NewLangDetector("nodejs", "package.json", "js,ts,jsx,tsx", `"node": "`)
	found, meta, err := detector.ScanFiles(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected found=true (js files exist)")
	}
	// No package.json means no version detection - meta should be nil
	if meta != nil {
		t.Errorf("expected meta=nil when no package.json, got %v", meta)
	}
}

func TestLangDetector_IgnoresVendorAndNodeModules(t *testing.T) {
	t.Parallel()

	// Files in vendor/ and node_modules/ should be ignored
	fm := newStubFileManager(map[string]string{
		"go.mod":             "module example.com/test\n\ngo 1.21\n",
		"main.go":            "package main\n",
		"vendor/go.mod":      "module vendored\n\ngo 1.18\n",
		"vendor/vendored.go": "package vendored\n",
	})

	detector := NewLangDetector("golang", "go.mod", "go", "go ")
	found, meta, err := detector.ScanFiles(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected found=true")
	}
	// Should use 1.21 from root, not 1.18 from vendor
	// MaxVersionFromConstraints normalizes to semver format
	if meta["version"] != "1.21.0" {
		t.Errorf("expected version=1.21.0 (ignoring vendor), got %s", meta["version"])
	}
}

func TestLangDetector_NoFilesAtAll(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{})

	detector := NewLangDetector("golang", "go.mod", "go", "go ")
	found, _, err := detector.ScanFiles(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected found=false for empty directory")
	}
}

func TestLangDetector_ErrorsOnMissingTargetAndExtensions(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{})

	// Both targetFile and fileExtensions are empty - should error
	detector := NewLangDetector("test", "", "", "prefix")
	_, _, err := detector.ScanFiles(fm)

	if err == nil {
		t.Error("expected error when targetFile and fileExtensions are both empty")
	}
}
