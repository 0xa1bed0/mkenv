package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ComposeImageTag returns a Docker-safe tag from an optional prefix and two hex cache keys.
// Result is either "<prefix>-<64-hex>" (prefix ≤ 63 chars after sanitization) or just "<64-hex>".
func composeImageTag(prefix string, a, b CacheKey) string {
	// Decode hex -> raw bytes; be defensive if inputs aren't valid hex.
	ah, errA := hex.DecodeString(string(a))
	if errA != nil {
		ah = []byte(a)
	}
	bh, errB := hex.DecodeString(string(b))
	if errB != nil {
		bh = []byte(b)
	}

	// Length-prefix and hash raw bytes of both parts to avoid ambiguity.
	h := sha256.New()
	var len8 [8]byte
	putU64 := func(n int) {
		len8[0] = byte(n >> 56)
		len8[1] = byte(n >> 48)
		len8[2] = byte(n >> 40)
		len8[3] = byte(n >> 32)
		len8[4] = byte(n >> 24)
		len8[5] = byte(n >> 16)
		len8[6] = byte(n >> 8)
		len8[7] = byte(n)
		h.Write(len8[:])
	}
	putU64(len(ah))
	h.Write(ah)
	putU64(len(bh))
	h.Write(bh)
	core := hex.EncodeToString(h.Sum(nil)) // 64 chars

	pfx := sanitizeTagPrefix(prefix)
	if pfx == "" {
		return core
	}

	// Enforce overall 128-char limit: "<pfx>-<core>" => pfx ≤ 63.
	if len(pfx) > 63 {
		pfx = pfx[:63]
	}
	return pfx + "-" + core
}

// composePrefix takes an absolute project path and returns a short, Docker-safe
// prefix derived from its last one or two directories. Example:
//
//	/Users/anatolii/projects/iusevimbtw/api        → iusevimbtw_api
//	/Users/anatolii/projects/iusevimbtw/api/file.js → iusevimbtw_api
//	/Users/anatolii/iusevimbtw                     → iusevimbtw
//
// The home directory is trimmed, and the result contains only letters,
// digits, underscores, and hyphens.
func composePrefix(projectPath string) string {
	if projectPath == "" {
		return "unknown-project"
	}

	// Expand ~ and clean path
	if strings.HasPrefix(projectPath, "~") {
		projectPath = strings.TrimPrefix(projectPath, "~")
		if home, err := os.UserHomeDir(); err == nil {
			projectPath = filepath.Join(home, projectPath)
		}
	}
	projectPath = filepath.Clean(projectPath)

	// Trim home directory prefix
	if home, err := os.UserHomeDir(); err == nil {
		if after, ok := strings.CutPrefix(projectPath, home); ok {
			projectPath = after
		}
	}

	// Split path and ignore empty segments
	parts := strings.FieldsFunc(projectPath, func(r rune) bool {
		return r == filepath.Separator
	})
	if len(parts) == 0 {
		return "unknown-project"
	}

	// If last segment is a file (has extension), drop it
	last := parts[len(parts)-1]
	if strings.ContainsRune(last, '.') {
		parts = parts[:len(parts)-1]
	}

	// Take up to two trailing dirs
	var elems []string
	if len(parts) >= 2 {
		elems = parts[len(parts)-2:]
	} else {
		elems = parts
	}

	prefix := strings.Join(elems, "_")
	prefix = sanitizeTagPrefix(prefix)

	if prefix == "" {
		return "unknown-project"
	}
	return prefix
}

// sanitizeTagPrefix keeps only [A-Za-z0-9_.-], lowercases, trims leading '.'/'-'.
// Returns "" if nothing valid remains.
func sanitizeTagPrefix(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			// else drop
		}
	}
	out := b.String()
	out = strings.TrimLeft(out, ".-")
	return out
}
