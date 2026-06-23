package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPreferencesFile_ParsesSetup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mkenv")
	content := `{
		"volumes": ["~/dotconfigs:~/dotconfigs"],
		"setup": ["~/dotconfigs/install.sh", "echo done"]
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	ec, err := loadPreferencesFile(path)
	if err != nil {
		t.Fatalf("loadPreferencesFile: %v", err)
	}

	got := ec.Setup()
	want := []string{"~/dotconfigs/install.sh", "echo done"}
	if len(got) != len(want) {
		t.Fatalf("Setup() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Setup()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMerge_AppendsSetup(t *testing.T) {
	base := buildDefaultEnvConfig()
	base.Setup_ = []string{"a"}

	src := buildDefaultEnvConfig()
	src.name = "child/.mkenv"
	src.Setup_ = []string{"b", "c"}

	base.Merge(src)

	got := base.Setup()
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("merged Setup() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("merged Setup()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// Setup commands are delivered at run time and never baked into the image, so
// changing them must not change the image signature (cache key) — otherwise
// editing setup would needlessly force a rebuild.
func TestSignature_IgnoresSetup(t *testing.T) {
	withSetup := buildDefaultEnvConfig()
	withSetup.Setup_ = []string{"~/dotconfigs/install.sh"}

	withoutSetup := buildDefaultEnvConfig()

	sigA, err := withSetup.Signature()
	if err != nil {
		t.Fatal(err)
	}
	sigB, err := withoutSetup.Signature()
	if err != nil {
		t.Fatal(err)
	}

	if sigA != sigB {
		t.Errorf("signature changed by setup commands: %s != %s", sigA, sigB)
	}
}
