// Package versions offers helpers for comparing semantic-like version strings.
package versions

import (
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

// MaxVersion returns the numerically largest version string.
// Supports arbitrary numbers of dot-separated numeric segments like "1.2.3.4".
// Returns an error if no valid versions found.
func MaxVersion(vs []string) (string, error) {
	return pickVersion(vs, true)
}

// MinVersion returns the numerically smallest version string.
// Supports arbitrary numbers of dot-separated numeric segments like "1.2.3.4".
// Returns an error if no valid versions found.
func MinVersion(vs []string) (string, error) {
	return pickVersion(vs, false)
}

// pickVersion is a shared helper for MaxVersion and MinVersion.
// If wantMax=true, finds the largest; otherwise finds the smallest.
func pickVersion(vs []string, wantMax bool) (string, error) {
	if len(vs) == 0 {
		return "", fmt.Errorf("no versions provided")
	}

	var best string
	var found bool

	for _, v := range vs {
		if v == "" {
			continue
		}
		if !isValidVersion(v) {
			return "", fmt.Errorf("invalid version: %q", v)
		}
		if !found {
			best = v
			found = true
			continue
		}
		cmp := compareVersions(v, best)
		if (wantMax && cmp > 0) || (!wantMax && cmp < 0) {
			best = v
		}
	}

	if !found {
		return "", fmt.Errorf("no valid versions found")
	}
	return best, nil
}

// compareVersions returns 1 if a > b, -1 if a < b, 0 if equal.
// Comparison is numeric per segment; missing segments are treated as 0.
// Example: "1.10" > "1.2"; "1.2" == "1.2.0"
func compareVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := max(len(as), len(bs))

	for i := 0; i < n; i++ {
		sa, sb := "0", "0"
		if i < len(as) {
			sa = as[i]
		}
		if i < len(bs) {
			sb = bs[i]
		}

		ai, bi := new(big.Int), new(big.Int)
		ai.SetString(sa, 10)
		bi.SetString(sb, 10)

		switch ai.Cmp(bi) {
		case 1:
			return 1
		case -1:
			return -1
		}
	}
	return 0
}

// isValidVersion ensures only digits and dots, and no empty segments like "1..2".
func isValidVersion(v string) bool {
	if v == "" {
		return false
	}
	parts := strings.Split(v, ".")
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if !unicode.IsDigit(r) {
				return false
			}
		}
	}
	return true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
