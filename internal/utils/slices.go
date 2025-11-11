package utils

import (
	"fmt"
	"sort"
	"strings"
)

func UniqueTrimmedStrings(input []string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, s := range input {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			continue // optional: skip empty strings
		}
		if _, exists := seen[trimmed]; !exists {
			seen[trimmed] = struct{}{}
			result = append(result, trimmed)
		}
	}

	return result
}

func WrapInQuotes(input []string) []string {
	var result []string

	for _, s := range input {
		wrapped := fmt.Sprintf("\"%s\"", strings.TrimSpace(s))
		result = append(result, wrapped)
	}

	return result
}

// Sorted unique helper for brick IDs (used by Dockerfile label generation later).
func UniqueSorted(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	cp := make([]string, len(ids))
	copy(cp, ids)
	sort.Strings(cp)

	out := make([]string, 0, len(cp))
	for i := range cp {
		if i == 0 || cp[i] != cp[i-1] {
			out = append(out, cp[i])
		}
	}
	return out
}
