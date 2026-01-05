package agentdist

import (
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Embed gzipped linux agents.
//
//go:embed bin/linux_amd64/mkenv-agent.gz
//go:embed bin/linux_arm64/mkenv-agent.gz
var fs embed.FS

// AgentSpec describes which embedded agent to use.
type AgentSpec struct {
	GOARCH string // linux arch: "amd64" or "arm64"
}

// ResolveAgentSpec chooses which linux agent to inject.
// If you already resolve platform elsewhere, pass it in and skip runtime guesses.
func resolveAgentSpec() (AgentSpec, error) {
	arch := runtime.GOARCH

	switch arch {
	case "amd64", "arm64":
		return AgentSpec{GOARCH: arch}, nil
	default:
		return AgentSpec{}, fmt.Errorf("unsupported linux arch for agent: %s", arch)
	}
}

// ExtractAgent ensures the matching embedded agent exists at dstPath.
// It is atomic and cached: re-extracts only if content hash differs.
func ExtractAgent(dstPath string) error {
	spec, err := resolveAgentSpec()
	if err != nil {
		return err
	}

	embeddedPath := embeddedAgentPath(spec.GOARCH)
	dstPath = dstPath + "/mkenv"

	// read embedded gz
	gzFile, err := fs.Open(embeddedPath)
	if err != nil {
		return fmt.Errorf("open embedded agent %q: %w", embeddedPath, err)
	}
	defer gzFile.Close()

	// compute hash of embedded gz (as cache key)
	embeddedBytes, err := io.ReadAll(gzFile)
	if err != nil {
		return fmt.Errorf("read embedded agent %q: %w", embeddedPath, err)
	}
	embeddedHash := sha256.Sum256(embeddedBytes)
	hashHex := hex.EncodeToString(embeddedHash[:])

	// if present and hash matches, skip
	if ok, _ := hashMatches(dstPath, hashHex); ok {
		return nil
	}

	err = os.MkdirAll(filepath.Dir(dstPath), 0o755)
	if err != nil {
		return err
	}

	// re-open for actual unzip
	gzReader, err := gzip.NewReader(strings.NewReader(string(embeddedBytes)))
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gzReader.Close()

	tmp := dstPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, gzReader); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, dstPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	// write sidecar hash
	return os.WriteFile(dstPath+".sha256", []byte(hashHex), 0o644)
}

func embeddedAgentPath(goarch string) string {
	switch goarch {
	case "amd64":
		return "bin/linux_amd64/mkenv-agent.gz"
	case "arm64":
		return "bin/linux_arm64/mkenv-agent.gz"
	default:
		// guarded earlier
		return ""
	}
}

func hashMatches(dstPath, embeddedHashHex string) (bool, error) {
	sidecar := dstPath + ".sha256"
	b, err := os.ReadFile(sidecar)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(b)) == embeddedHashHex, nil
}
