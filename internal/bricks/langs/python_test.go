package langs

import (
	"testing"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

func createPythonDetector() *pythonDetector {
	return &pythonDetector{
		pythonVersionDetector: bricksengine.NewLangDetector(
			string(pythonID), ".python-version", "py", "",
			bricksengine.WithVersionSemantics(bricksengine.VersionSemanticsMinimum),
		),
		langDetector: bricksengine.NewLangDetector(string(pythonID), "requirements.txt,pyproject.toml,setup.py,Pipfile", "py", `python_requires`),
	}
}

func TestPythonDetector_PythonVersionFilePriority(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		".python-version": "3.12.1\n",
		"pyproject.toml":  `[project]\nrequires-python = ">=3.11"\npython_requires = ">=3.11"\n`,
		"main.py":         "print('hello')\n",
	})

	detector := createPythonDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != pythonID {
		t.Errorf("expected brickID=%s, got %s", pythonID, brickID)
	}
	// .python-version is priority — should use 3.12.1
	if meta["version"] != "3.12.1" {
		t.Errorf("expected version=3.12.1 (.python-version priority), got %s", meta["version"])
	}
}

func TestPythonDetector_NoPythonVersionFile_FallbackDetectsLanguage(t *testing.T) {
	t.Parallel()

	// No .python-version file — should fall back to langDetector which
	// detects python via .py extensions and target files
	fm := newStubFileManager(map[string]string{
		"requirements.txt": "flask>=2.0\n",
		"main.py":          "print('hello')\n",
	})

	detector := createPythonDetector()
	brickID, _, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != pythonID {
		t.Errorf("expected brickID=%s, got %s", pythonID, brickID)
	}
}

func TestPythonDetector_OnlyPyFiles(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"main.py": "print('hello')\n",
	})

	detector := createPythonDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != pythonID {
		t.Errorf("expected brickID=%s, got %s", pythonID, brickID)
	}
	// No version source — meta should have no version
	if meta != nil && meta["version"] != "" {
		t.Errorf("expected no version, got %s", meta["version"])
	}
}

func TestPythonDetector_NoFiles(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"index.js": "console.log('hello');\n",
	})

	detector := createPythonDetector()
	brickID, _, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != "" {
		t.Errorf("expected empty brickID for non-python project, got %s", brickID)
	}
}

func TestPythonDetector_PythonVersionFileOnly(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		".python-version": "3.10.5\n",
		"main.py":         "print('hello')\n",
	})

	detector := createPythonDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != pythonID {
		t.Errorf("expected brickID=%s, got %s", pythonID, brickID)
	}
	if meta["version"] != "3.10.5" {
		t.Errorf("expected version=3.10.5, got %s", meta["version"])
	}
}
