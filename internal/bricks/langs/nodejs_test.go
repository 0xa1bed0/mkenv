package langs

import (
	"bufio"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

// stubFileManager is an in-memory implementation of FileManager for testing.
type stubFileManager struct {
	files map[string]string
}

func newStubFileManager(files map[string]string) *stubFileManager {
	return &stubFileManager{files: files}
}

func (s *stubFileManager) FindFile(filename string, ignorePaths []string) ([]string, error) {
	var results []string
	for path := range s.files {
		if filepath.Base(path) == filename {
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

func createNodejsDetector() *nodejsDetector {
	return &nodejsDetector{
		packageJsonDetector: bricksengine.NewLangDetector(string(nodejsID), "package.json", "html,htm,htmlx,htmx,js,ts,jsx", `"node": "`),
		npmrcDetector:       bricksengine.NewLangDetector(string(nodejsID), ".npmrc", "html,htm,htmlx,htmx,js,ts,jsx", "node-version="),
	}
}

func TestNodejsDetector_NpmrcWithNodeVersion(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		".npmrc":   "registry=https://registry.npmjs.org/\nnode-version=18.0.0\n",
		"index.js": "console.log('hello');\n", // Need JS file to trigger detection
	})

	detector := createNodejsDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != nodejsID {
		t.Errorf("expected brickID=%s, got %s", nodejsID, brickID)
	}
	if meta["version"] != "18.0.0" {
		t.Errorf("expected version=18.0.0, got %s", meta["version"])
	}
}

func TestNodejsDetector_NpmrcWithoutNodeVersion(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		".npmrc":   "registry=https://registry.npmjs.org/\nsave-exact=true\n",
		"index.js": "console.log('hello');\n",
	})

	detector := createNodejsDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != nodejsID {
		t.Errorf("expected brickID=%s, got %s", nodejsID, brickID)
	}
	// No version in .npmrc and no package.json with engine - meta may be nil
	if meta != nil && meta["version"] != "" {
		t.Logf("version found: %s", meta["version"])
	}
}

func TestNodejsDetector_PackageJsonAndNpmrc(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"package.json": `{
  "name": "test-project",
  "engines": {
    "node": ">=16.0.0"
  }
}`,
		".npmrc":   "node-version=18.0.0\n",
		"index.js": "console.log('hello');\n",
	})

	detector := createNodejsDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != nodejsID {
		t.Errorf("expected brickID=%s, got %s", nodejsID, brickID)
	}
	// Should pick max of 16.0.0 and 18.0.0
	if meta["version"] != "18.0.0" {
		t.Errorf("expected version=18.0.0 (max of package.json and .npmrc), got %s", meta["version"])
	}
}

func TestNodejsDetector_NpmrcInNodeModulesIgnored(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"node_modules/.npmrc": "node-version=22.0.0\n",
		"index.js":            "console.log('hello');\n",
	})

	detector := createNodejsDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still detect nodejs due to .js file
	if brickID != nodejsID {
		t.Errorf("expected brickID=%s, got %s", nodejsID, brickID)
	}
	// But version from node_modules/.npmrc should be ignored
	if meta != nil && meta["version"] == "22.0.0" {
		t.Error("version from node_modules/.npmrc should have been ignored")
	}
}

func TestNodejsDetector_OnlyJSFiles(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"index.js": "console.log('hello');\n",
		"lib.js":   "module.exports = {};\n",
	})

	detector := createNodejsDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should detect nodejs due to .js files
	if brickID != nodejsID {
		t.Errorf("expected brickID=%s, got %s", nodejsID, brickID)
	}
	// No version source available
	if meta != nil && meta["version"] != "" {
		t.Logf("version found: %s", meta["version"])
	}
}

func TestNodejsDetector_NpmrcVersionWithConstraint(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		".npmrc":   "node-version=>=18.0.0\n",
		"index.js": "console.log('hello');\n",
	})

	detector := createNodejsDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != nodejsID {
		t.Errorf("expected brickID=%s, got %s", nodejsID, brickID)
	}
	// MaxVersionFromConstraints resolves constraint to minimum version
	if meta["version"] != "18.0.0" {
		t.Errorf("expected version=18.0.0, got %s", meta["version"])
	}
}

func TestNodejsDetector_NoFiles(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"main.go": "package main\n", // Only Go file, no JS
	})

	detector := createNodejsDetector()
	brickID, _, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != "" {
		t.Errorf("expected empty brickID for non-nodejs project, got %s", brickID)
	}
}
