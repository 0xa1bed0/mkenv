package cache

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
)

// CacheKeyDockerfileLines deterministically computes a cache key for a list of Dockerfile lines.
// It prefixes each line with its length (8-byte big-endian) before hashing to avoid collisions
// between sequences like ["ab", "c"] and ["a", "bc"].
func CacheKeyDockerfileLines(lines []string) CacheKey {
	h := sha256.New()
	var lenBuf [8]byte

	for _, line := range lines {
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(line)))
		h.Write(lenBuf[:])
		io.WriteString(h, line)
	}

	return CacheKey(hex.EncodeToString(h.Sum(nil)))
}
