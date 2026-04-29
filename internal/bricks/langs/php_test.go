package langs

import (
	"testing"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

func createPHPDetector() *phpDetector {
	return &phpDetector{
		phpVersionDetector: bricksengine.NewLangDetector(
			string(phpID), ".php-version", "php", "",
			bricksengine.WithVersionSemantics(bricksengine.VersionSemanticsMinimum),
		),
		langDetector: bricksengine.NewLangDetector(string(phpID), "composer.json", "php", `"php": "`),
	}
}

func TestPHPDetector_PhpVersionFilePriority(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		".php-version": "8.2.1\n",
		"composer.json": `{
  "require": {
    "php": ">=8.1"
  }
}`,
		"index.php": "<?php echo 'hello'; ?>\n",
	})

	detector := createPHPDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != phpID {
		t.Errorf("expected brickID=%s, got %s", phpID, brickID)
	}
	// .php-version is priority — should use 8.2.1
	if meta["version"] != "8.2.1" {
		t.Errorf("expected version=8.2.1 (.php-version priority), got %s", meta["version"])
	}
}

func TestPHPDetector_NoPhpVersionFile_Fallback(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"composer.json": `{
  "require": {
    "php": ">=8.1"
  }
}`,
		"index.php": "<?php echo 'hello'; ?>\n",
	})

	detector := createPHPDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != phpID {
		t.Errorf("expected brickID=%s, got %s", phpID, brickID)
	}
	// Should fall through to composer.json
	if meta["version"] != "8.1.0" {
		t.Errorf("expected version=8.1.0 (fallback from composer.json), got %s", meta["version"])
	}
}

func TestPHPDetector_OnlyPhpFiles(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"index.php": "<?php echo 'hello'; ?>\n",
	})

	detector := createPHPDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != phpID {
		t.Errorf("expected brickID=%s, got %s", phpID, brickID)
	}
	// No version source — meta should have no version
	if meta != nil && meta["version"] != "" {
		t.Errorf("expected no version, got %s", meta["version"])
	}
}

func TestPHPDetector_NoFiles(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"index.js": "console.log('hello');\n",
	})

	detector := createPHPDetector()
	brickID, _, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != "" {
		t.Errorf("expected empty brickID for non-php project, got %s", brickID)
	}
}

func TestPHPDetector_PhpVersionFileOnly(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		".php-version": "8.3.0\n",
		"index.php":    "<?php echo 'hello'; ?>\n",
	})

	detector := createPHPDetector()
	brickID, meta, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != phpID {
		t.Errorf("expected brickID=%s, got %s", phpID, brickID)
	}
	if meta["version"] != "8.3.0" {
		t.Errorf("expected version=8.3.0, got %s", meta["version"])
	}
}
