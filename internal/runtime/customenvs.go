package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/guardrails"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/utils"
)

// SecretFromFilePrefix marks a custom env var value as "read from this host file
// at container-start time". The remainder of the value after the prefix is the
// file path. Supports absolute paths and "~/..." (home-relative). Relative paths
// are rejected to remove ambiguity.
//
// Reserved: a literal env value cannot start with this prefix.
const SecretFromFilePrefix = "mkenv_value_from:"

// maxSecretFileSize is the hard cap on host file content that may be loaded
// into a single env var. Prevents a misconfigured/large file from being loaded
// into the container's env block (Linux execve env+argv has a strict size limit
// — typically 128 KiB — and very large env vars cause obscure failures).
const maxSecretFileSize = 1 << 20 // 1 MiB

// maxEnvNameLen caps custom env var name length to a sane value.
const maxEnvNameLen = 256

// envNameRE matches valid POSIX-shell-compatible env var names: an initial
// letter or underscore, then letters/digits/underscores. Lowercase is allowed
// but discouraged.
var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reservedEnvPrefixes are name prefixes that mkenv owns; user-defined envs
// using them would collide with internal vars (e.g. MKENV_REVERSE_PROXY).
var reservedEnvPrefixes = []string{"MKENV_"}

// dangerousEnvNames are loader/runtime knobs that an environment variable can
// abuse to hijack execution inside the container. They are blocked so that
// a stray .mkenv file (e.g. inherited from a parent directory) cannot quietly
// inject a preload library or alter the dynamic loader's search path.
var dangerousEnvNames = map[string]struct{}{
	"LD_PRELOAD":                 {},
	"LD_LIBRARY_PATH":            {},
	"LD_AUDIT":                   {},
	"DYLD_INSERT_LIBRARIES":      {},
	"DYLD_LIBRARY_PATH":          {},
	"DYLD_FALLBACK_LIBRARY_PATH": {},
}

var (
	ErrInvalidEnvName    = errors.New("invalid env var name")
	ErrReservedEnvName   = errors.New("env var name is reserved by mkenv")
	ErrDangerousEnvName  = errors.New("env var name is on the dangerous-name blocklist")
	ErrSecretPathUnsafe  = errors.New("secret file path is unsafe")
	ErrSecretPathBlocked = errors.New("secret file path is blocked by guardrails")
	ErrSecretFileTooBig  = errors.New("secret file exceeds size cap")
	ErrSecretFileEmpty   = errors.New("secret file resolved to empty value")
	ErrNullByte          = errors.New("env var contains NUL byte")
)

// ResolveCustomEnvs validates and resolves the raw envs map from .mkenv into
// docker-runtime-style "KEY=VALUE" strings. Returns a deterministic slice
// (sorted by key) so cache-key/log behavior is stable.
//
// Resolution failures are returned as errors — callers MUST fail-closed and not
// start the container with a partial env set.
//
// Security posture (the dangerous bit — read carefully before changing):
//
//  1. Names are validated against a strict regex and an explicit blocklist.
//     Reserved MKENV_* and loader-hijack names (LD_PRELOAD etc.) are rejected.
//  2. Literal values are scanned for NUL bytes (docker rejects them, but we
//     surface a clear error rather than a cryptic docker failure).
//  3. The "mkenv_value_from:<path>" directive:
//     a. Requires absolute or "~/..." path. Relative paths are rejected to
//     remove cwd-vs-mkenv-file-vs-project-root ambiguity.
//     b. Path is symlink-resolved with utils.ResolvePathStrict, then checked
//     against guardrails.IsAbsolutelyForbidden. A symlink that escapes
//     into ~/.ssh, ~/.aws, /etc, /dev, etc. is rejected POST-resolution,
//     so symlink-based escape is closed.
//     c. The file is os.Lstat'd to assert it is a regular file (no fifo /
//     device / directory).
//     d. Read is capped at maxSecretFileSize bytes — even if Stat lied,
//     io.LimitReader prevents unbounded consumption.
//     e. Content must not contain a NUL byte.
//     f. World/group-readable files trigger a loud warning (the user has
//     stored a credential carelessly), but do not block.
//     g. Exactly one trailing newline (\n or \r\n) is trimmed — files
//     created with `echo "tok" > f` have one. Internal newlines (PEM
//     keys, multi-line tokens) are preserved.
//     h. Empty resolved values are rejected (almost always a misconfig).
//  4. Resolved secret values are never logged. Only the key is logged.
func ResolveCustomEnvs(raw map[string]string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	// Sort for deterministic output (helps tests and reproducible logging).
	sortStrings(keys)

	out := make([]string, 0, len(keys))
	for _, name := range keys {
		if err := validateEnvName(name); err != nil {
			return nil, fmt.Errorf("env %q: %w", name, err)
		}

		spec := raw[name]
		value, isSecret, err := resolveEnvValue(name, spec)
		if err != nil {
			// Error message never embeds the resolved value.
			return nil, fmt.Errorf("env %q: %w", name, err)
		}
		if strings.ContainsRune(value, 0) {
			return nil, fmt.Errorf("env %q: %w", name, ErrNullByte)
		}

		if isSecret {
			logs.Debugf("Custom env %s resolved from secret file (%d bytes)", name, len(value))
		} else {
			logs.Debugf("Custom env %s set from literal", name)
		}
		out = append(out, name+"="+value)
	}
	return out, nil
}

func validateEnvName(name string) error {
	if len(name) == 0 || len(name) > maxEnvNameLen {
		return ErrInvalidEnvName
	}
	if strings.ContainsRune(name, 0) {
		return ErrNullByte
	}
	if !envNameRE.MatchString(name) {
		return ErrInvalidEnvName
	}
	upper := strings.ToUpper(name)
	for _, p := range reservedEnvPrefixes {
		if strings.HasPrefix(upper, p) {
			return ErrReservedEnvName
		}
	}
	if _, bad := dangerousEnvNames[upper]; bad {
		return ErrDangerousEnvName
	}
	return nil
}

// resolveEnvValue returns (value, isSecret, error). isSecret is true when the
// value came from a mkenv_value_from: file (used to suppress logging).
func resolveEnvValue(name, spec string) (string, bool, error) {
	if !strings.HasPrefix(spec, SecretFromFilePrefix) {
		return spec, false, nil
	}

	rawPath := strings.TrimSpace(spec[len(SecretFromFilePrefix):])
	if rawPath == "" {
		return "", true, fmt.Errorf("%w: empty path after %q", ErrSecretPathUnsafe, SecretFromFilePrefix)
	}
	if strings.ContainsRune(rawPath, 0) {
		return "", true, fmt.Errorf("%w: NUL byte in path", ErrSecretPathUnsafe)
	}

	// Require absolute or "~/..." — no relative paths.
	if !strings.HasPrefix(rawPath, "/") && !strings.HasPrefix(rawPath, "~/") && rawPath != "~" {
		return "", true, fmt.Errorf("%w: path must be absolute or start with ~/", ErrSecretPathUnsafe)
	}

	resolved, err := utils.ResolvePathStrict(rawPath)
	if err != nil {
		// Don't leak the rawPath if it might be sensitive; the user typed it,
		// so echoing it back is fine and aids debugging.
		return "", true, fmt.Errorf("%w: resolve %q: %v", ErrSecretPathUnsafe, rawPath, err)
	}

	// Symlink-resolved path is now checked against the same forbidden-path rules
	// mkenv applies to mounts. This closes "symlink escape into ~/.ssh".
	if guardrails.IsAbsolutelyForbidden(resolved) {
		return "", true, fmt.Errorf("%w: %q resolves to a path mkenv refuses to read", ErrSecretPathBlocked, rawPath)
	}

	fi, err := os.Lstat(resolved)
	if err != nil {
		return "", true, fmt.Errorf("%w: stat: %v", ErrSecretPathUnsafe, err)
	}
	// EvalSymlinks already followed links. After that, Lstat on the final path
	// should report a regular file. If it doesn't, refuse — we never want to
	// read from a fifo, char device, block device, socket, or directory.
	if !fi.Mode().IsRegular() {
		return "", true, fmt.Errorf("%w: %q is not a regular file", ErrSecretPathUnsafe, rawPath)
	}
	if fi.Size() > maxSecretFileSize {
		return "", true, fmt.Errorf("%w: %q is %d bytes (cap %d)", ErrSecretFileTooBig, rawPath, fi.Size(), maxSecretFileSize)
	}

	// Loud warning for promiscuous perms. We do NOT block — the user may have
	// intentional reasons (e.g. CI shared credential). But this is the kind of
	// thing security-conscious users want to know about.
	if mode := fi.Mode().Perm(); mode&0o077 != 0 {
		logs.Warnf("secret file %s is readable by group/other (mode %04o) — consider chmod 600", rawPath, mode)
	}

	f, err := os.Open(resolved)
	if err != nil {
		return "", true, fmt.Errorf("%w: open: %v", ErrSecretPathUnsafe, err)
	}
	defer f.Close()

	// LimitReader caps actual bytes consumed even if Stat reported a smaller
	// size (defense in depth against /proc-like virtual files, TOCTOU).
	buf, err := io.ReadAll(io.LimitReader(f, maxSecretFileSize+1))
	if err != nil {
		return "", true, fmt.Errorf("%w: read: %v", ErrSecretPathUnsafe, err)
	}
	if len(buf) > maxSecretFileSize {
		return "", true, fmt.Errorf("%w: %q exceeds %d bytes during read", ErrSecretFileTooBig, rawPath, maxSecretFileSize)
	}

	// NUL bytes anywhere → docker would reject; surface a clean error.
	if bytes.IndexByte(buf, 0) >= 0 {
		return "", true, fmt.Errorf("%w: in %q", ErrNullByte, rawPath)
	}

	// Trim exactly one trailing newline (typical of `echo foo > f`).
	if len(buf) > 0 && buf[len(buf)-1] == '\n' {
		buf = buf[:len(buf)-1]
		if len(buf) > 0 && buf[len(buf)-1] == '\r' {
			buf = buf[:len(buf)-1]
		}
	}

	if len(buf) == 0 {
		return "", true, fmt.Errorf("%w: %q", ErrSecretFileEmpty, rawPath)
	}
	return string(buf), true, nil
}

func sortStrings(s []string) {
	// Tiny in-place insertion sort — avoids pulling sort just for this and
	// keeps allocations zero for typical .mkenv sizes (<20 keys).
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
