package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCustomEnvs_Empty(t *testing.T) {
	got, err := ResolveCustomEnvs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
	got, err = ResolveCustomEnvs(map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestResolveCustomEnvs_LiteralValues(t *testing.T) {
	got, err := ResolveCustomEnvs(map[string]string{
		"FOO":      "bar",
		"BAZ":      "qux",
		"EMPTY_OK": "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// sorted order
	want := []string{"BAZ=qux", "EMPTY_OK=", "FOO=bar"}
	if !slicesEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResolveCustomEnvs_InvalidName(t *testing.T) {
	cases := map[string]string{
		"starts-with-digit": "1FOO",
		"dash":              "FOO-BAR",
		"space":             "FOO BAR",
		"empty":             "",
		"too-long":          strings.Repeat("A", 257),
	}
	for label, name := range cases {
		_, err := ResolveCustomEnvs(map[string]string{name: "v"})
		if !errors.Is(err, ErrInvalidEnvName) {
			t.Errorf("%s: expected ErrInvalidEnvName, got %v", label, err)
		}
	}
}

func TestResolveCustomEnvs_NullByteInName(t *testing.T) {
	_, err := ResolveCustomEnvs(map[string]string{"FOO\x00BAR": "v"})
	// validateEnvName checks regex first; a NUL fails the regex too. Both errors
	// indicate "we refuse this name" which is what matters.
	if !errors.Is(err, ErrNullByte) && !errors.Is(err, ErrInvalidEnvName) {
		t.Fatalf("expected ErrNullByte or ErrInvalidEnvName, got %v", err)
	}
}

func TestResolveCustomEnvs_ReservedPrefix(t *testing.T) {
	_, err := ResolveCustomEnvs(map[string]string{"MKENV_INTERNAL": "v"})
	if !errors.Is(err, ErrReservedEnvName) {
		t.Fatalf("expected ErrReservedEnvName, got %v", err)
	}
	// case-insensitive
	_, err = ResolveCustomEnvs(map[string]string{"mkenv_lower": "v"})
	if !errors.Is(err, ErrReservedEnvName) {
		t.Fatalf("expected ErrReservedEnvName, got %v", err)
	}
}

func TestResolveCustomEnvs_DangerousNames(t *testing.T) {
	for _, name := range []string{
		"LD_PRELOAD",
		"LD_LIBRARY_PATH",
		"LD_AUDIT",
		"DYLD_INSERT_LIBRARIES",
		"DYLD_LIBRARY_PATH",
		"DYLD_FALLBACK_LIBRARY_PATH",
	} {
		_, err := ResolveCustomEnvs(map[string]string{name: "/tmp/evil.so"})
		if !errors.Is(err, ErrDangerousEnvName) {
			t.Errorf("%s: expected ErrDangerousEnvName, got %v", name, err)
		}
	}
	// case-insensitive check
	_, err := ResolveCustomEnvs(map[string]string{"ld_preload": "/tmp/evil.so"})
	if !errors.Is(err, ErrDangerousEnvName) {
		t.Errorf("ld_preload: expected ErrDangerousEnvName, got %v", err)
	}
}

func TestResolveCustomEnvs_NullByteInLiteral(t *testing.T) {
	_, err := ResolveCustomEnvs(map[string]string{"FOO": "a\x00b"})
	if !errors.Is(err, ErrNullByte) {
		t.Fatalf("expected ErrNullByte, got %v", err)
	}
}

func TestResolveCustomEnvs_SecretFromAbsolutePath(t *testing.T) {
	dir := safeTempDir(t)
	path := filepath.Join(dir, "secret.txt")
	writeFile(t, path, []byte("hunter2\n"), 0o600)

	got, err := ResolveCustomEnvs(map[string]string{
		"TOKEN": SecretFromFilePrefix + path,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"TOKEN=hunter2"}
	if !slicesEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResolveCustomEnvs_SecretTrimsTrailingCRLF(t *testing.T) {
	dir := safeTempDir(t)
	path := filepath.Join(dir, "secret.txt")
	writeFile(t, path, []byte("hunter2\r\n"), 0o600)

	got, err := ResolveCustomEnvs(map[string]string{"TOKEN": SecretFromFilePrefix + path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0] != "TOKEN=hunter2" {
		t.Fatalf("got %q, want %q", got[0], "TOKEN=hunter2")
	}
}

func TestResolveCustomEnvs_SecretPreservesInternalNewlines(t *testing.T) {
	dir := safeTempDir(t)
	path := filepath.Join(dir, "key.pem")
	// PEM-style content — internal newlines must be preserved; only trailing trimmed.
	writeFile(t, path, []byte("-----BEGIN-----\nbody\n-----END-----\n"), 0o600)

	got, err := ResolveCustomEnvs(map[string]string{"PEM": SecretFromFilePrefix + path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "PEM=-----BEGIN-----\nbody\n-----END-----"
	if got[0] != want {
		t.Fatalf("got %q, want %q", got[0], want)
	}
}

func TestResolveCustomEnvs_RelativePathRejected(t *testing.T) {
	_, err := ResolveCustomEnvs(map[string]string{"TOKEN": SecretFromFilePrefix + "secret.txt"})
	if !errors.Is(err, ErrSecretPathUnsafe) {
		t.Fatalf("expected ErrSecretPathUnsafe, got %v", err)
	}
	_, err = ResolveCustomEnvs(map[string]string{"TOKEN": SecretFromFilePrefix + "./secret.txt"})
	if !errors.Is(err, ErrSecretPathUnsafe) {
		t.Fatalf("expected ErrSecretPathUnsafe for ./path, got %v", err)
	}
	_, err = ResolveCustomEnvs(map[string]string{"TOKEN": SecretFromFilePrefix + "../secret.txt"})
	if !errors.Is(err, ErrSecretPathUnsafe) {
		t.Fatalf("expected ErrSecretPathUnsafe for ../path, got %v", err)
	}
}

func TestResolveCustomEnvs_EmptyPath(t *testing.T) {
	_, err := ResolveCustomEnvs(map[string]string{"TOKEN": SecretFromFilePrefix})
	if !errors.Is(err, ErrSecretPathUnsafe) {
		t.Fatalf("expected ErrSecretPathUnsafe, got %v", err)
	}
	_, err = ResolveCustomEnvs(map[string]string{"TOKEN": SecretFromFilePrefix + "   "})
	if !errors.Is(err, ErrSecretPathUnsafe) {
		t.Fatalf("expected ErrSecretPathUnsafe for whitespace path, got %v", err)
	}
}

func TestResolveCustomEnvs_NullByteInPath(t *testing.T) {
	_, err := ResolveCustomEnvs(map[string]string{"TOKEN": SecretFromFilePrefix + "/tmp/foo\x00.txt"})
	if !errors.Is(err, ErrSecretPathUnsafe) {
		t.Fatalf("expected ErrSecretPathUnsafe, got %v", err)
	}
}

func TestResolveCustomEnvs_NonexistentFile(t *testing.T) {
	_, err := ResolveCustomEnvs(map[string]string{
		"TOKEN": SecretFromFilePrefix + "/tmp/this/definitely/does/not/exist-mkenv-test.xyz",
	})
	if !errors.Is(err, ErrSecretPathUnsafe) {
		t.Fatalf("expected ErrSecretPathUnsafe, got %v", err)
	}
}

func TestResolveCustomEnvs_DirectoryRejected(t *testing.T) {
	dir := safeTempDir(t)
	_, err := ResolveCustomEnvs(map[string]string{"TOKEN": SecretFromFilePrefix + dir})
	if !errors.Is(err, ErrSecretPathUnsafe) {
		t.Fatalf("expected ErrSecretPathUnsafe for directory, got %v", err)
	}
}

func TestResolveCustomEnvs_OversizedFile(t *testing.T) {
	dir := safeTempDir(t)
	path := filepath.Join(dir, "big.bin")
	writeFile(t, path, make([]byte, maxSecretFileSize+1), 0o600)

	_, err := ResolveCustomEnvs(map[string]string{"BIG": SecretFromFilePrefix + path})
	if !errors.Is(err, ErrSecretFileTooBig) {
		t.Fatalf("expected ErrSecretFileTooBig, got %v", err)
	}
}

func TestResolveCustomEnvs_NullByteInSecretContent(t *testing.T) {
	dir := safeTempDir(t)
	path := filepath.Join(dir, "binary.bin")
	writeFile(t, path, []byte("ab\x00cd"), 0o600)

	_, err := ResolveCustomEnvs(map[string]string{"BIN": SecretFromFilePrefix + path})
	if !errors.Is(err, ErrNullByte) {
		t.Fatalf("expected ErrNullByte, got %v", err)
	}
}

func TestResolveCustomEnvs_EmptySecretFile(t *testing.T) {
	dir := safeTempDir(t)
	path := filepath.Join(dir, "empty.txt")
	writeFile(t, path, nil, 0o600)

	_, err := ResolveCustomEnvs(map[string]string{"EMPTY": SecretFromFilePrefix + path})
	if !errors.Is(err, ErrSecretFileEmpty) {
		t.Fatalf("expected ErrSecretFileEmpty, got %v", err)
	}

	// File with only a newline → also empty after trim.
	path2 := filepath.Join(dir, "newline.txt")
	writeFile(t, path2, []byte("\n"), 0o600)
	_, err = ResolveCustomEnvs(map[string]string{"NL": SecretFromFilePrefix + path2})
	if !errors.Is(err, ErrSecretFileEmpty) {
		t.Fatalf("expected ErrSecretFileEmpty for newline-only file, got %v", err)
	}
}

// The critical guardrail test: a symlink that escapes into a forbidden system
// path (/etc) must be rejected POST symlink-resolution. If this fails, the
// resolver is exploitable: a user could ship a .mkenv that asks for
// "mkenv_value_from:~/innocent" where ~/innocent → /etc/shadow.
func TestResolveCustomEnvs_SymlinkEscapeBlocked(t *testing.T) {
	dir := safeTempDir(t)
	link := filepath.Join(dir, "innocent")
	// Symlink to /etc/hostname (a real regular file that's in the forbidden
	// /etc prefix). On macOS /etc is itself a symlink to /private/etc — both
	// are in the forbidden list, so this still blocks.
	if err := os.Symlink("/etc/hostname", link); err != nil {
		t.Skipf("cannot create symlink (likely Windows): %v", err)
	}

	_, err := ResolveCustomEnvs(map[string]string{"HACK": SecretFromFilePrefix + link})
	if err == nil {
		t.Fatalf("expected error for symlink escape, got nil")
	}
	if !errors.Is(err, ErrSecretPathBlocked) {
		t.Fatalf("expected ErrSecretPathBlocked, got %v", err)
	}
}

func TestResolveCustomEnvs_ForbiddenPathDirect(t *testing.T) {
	// Direct read attempt of a /etc file.
	if _, err := os.Stat("/etc/hostname"); err != nil {
		t.Skip("/etc/hostname not present — skipping")
	}
	_, err := ResolveCustomEnvs(map[string]string{"HOST": SecretFromFilePrefix + "/etc/hostname"})
	if !errors.Is(err, ErrSecretPathBlocked) {
		t.Fatalf("expected ErrSecretPathBlocked, got %v", err)
	}
}

func TestResolveCustomEnvs_LiteralAlongsideSecret(t *testing.T) {
	dir := safeTempDir(t)
	path := filepath.Join(dir, "secret.txt")
	writeFile(t, path, []byte("topsecret\n"), 0o600)

	got, err := ResolveCustomEnvs(map[string]string{
		"FOO":    "bar",
		"SECRET": SecretFromFilePrefix + path,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"FOO=bar", "SECRET=topsecret"}
	if !slicesEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// ---- helpers ----

// safeTempDir creates a temp directory under HOME (rather than t.TempDir which
// puts things in /tmp). /tmp is in the mkenv guardrails forbidden list — using
// it would cause every secret-file test to fail the post-resolution guardrail
// check for the wrong reason.
func safeTempDir(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	dir, err := os.MkdirTemp(home, "mkenv-customenvs-test-*")
	if err != nil {
		t.Fatalf("mkdirtemp under home: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
