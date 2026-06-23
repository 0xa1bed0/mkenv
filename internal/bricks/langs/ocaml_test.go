package langs

import (
	"testing"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

func createOCamlDetector() *ocamlDetector {
	return &ocamlDetector{
		langDetector: bricksengine.NewLangDetector(string(ocamlID), "dune-project", "ml,mli", ""),
	}
}

func TestOCamlDetector_DuneProject(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"dune-project": "(lang dune 3.0)\n",
		"main.ml":      "let () = print_endline \"hello\"\n",
	})

	detector := createOCamlDetector()
	brickID, _, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != ocamlID {
		t.Errorf("expected brickID=%s, got %s", ocamlID, brickID)
	}
}

func TestOCamlDetector_SourceFilesOnly(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"lib.ml":  "let answer = 42\n",
		"lib.mli": "val answer : int\n",
	})

	detector := createOCamlDetector()
	brickID, _, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != ocamlID {
		t.Errorf("expected brickID=%s, got %s", ocamlID, brickID)
	}
}

func TestOCamlDetector_NoFiles(t *testing.T) {
	t.Parallel()

	fm := newStubFileManager(map[string]string{
		"index.js": "console.log('hello');\n",
	})

	detector := createOCamlDetector()
	brickID, _, err := detector.Scan(fm)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brickID != "" {
		t.Errorf("expected empty brickID for non-ocaml project, got %s", brickID)
	}
}

func TestNewOCaml_DefaultVersion(t *testing.T) {
	t.Parallel()

	brick, err := NewOCaml(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brick.ID() != ocamlID {
		t.Errorf("expected brick ID=%s, got %s", ocamlID, brick.ID())
	}
}
