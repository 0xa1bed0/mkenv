package versions

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// ErrConflictingConstraints is the sentinel you can check with errors.Is.
var ErrConflictingConstraints = errors.New("conflicting version constraints")

// ConflictError indicates no candidate satisfied all constraints.
// The call still returns a best-effort chosen version.
type ConflictError struct {
	Constraints []string
	Candidates  []string
	Chosen      string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%v: no candidate satisfies all constraints; using %s", ErrConflictingConstraints, e.Chosen)
}

func (e *ConflictError) Unwrap() error { return ErrConflictingConstraints }

// MaxVersionFromConstraints returns the largest version that satisfies all given
// constraints, choosing only from the versions that are explicitly mentioned
// inside the constraint strings themselves.
//
// Examples of accepted constraints (npm-style):
//
//	">=16.0.0 <17.0.0"
//	"^18.12.1"
//	"~20"
//	">=18 <19 || >=20.10.0 <21"
//
// Notes:
//   - All constraint strings are ANDed together (every one must be satisfied).
//   - Candidate versions are collected by scanning the constraint text for
//     version-like literals. Partials are normalized (16 -> 16.0.0, 16.2 -> 16.2.0).
//   - Literals that include a pre-release (e.g., 20.0.0-rc.1) are kept as-is.
//   - If no candidate satisfies all constraints, an error is returned.
func MaxVersionFromConstraints(constraints []string) (string, error) {
	if len(constraints) == 0 {
		return "", errors.New("no constraints provided")
	}

	// Parse each constraint string (AND across the slice).
	parsed := make([]*semver.Constraints, 0, len(constraints))
	for _, c := range constraints {
		pc, err := semver.NewConstraint(c)
		if err != nil {
			return "", fmt.Errorf("invalid constraint %q: %w", c, err)
		}
		parsed = append(parsed, pc)
	}

	// Collect version literals mentioned anywhere in the constraint text.
	candidates := collectCandidates(constraints)
	if len(candidates) == 0 {
		return "", errors.New("no candidate versions found in constraints")
	}

	// Keep only candidates that satisfy ALL constraints (no loop labels).
	filtered := make([]*semver.Version, 0, len(candidates))
	for _, v := range candidates {
		ok := true
		for _, c := range parsed {
			if !c.Check(v) {
				ok = false
				break
			}
		}
		if ok {
			filtered = append(filtered, v)
		}
	}

	// Helper: pick max (assumes len > 0).
	pickMax := func(list []*semver.Version) *semver.Version {
		sort.Slice(list, func(i, j int) bool { return list[i].LessThan(list[j]) })
		return list[len(list)-1]
	}

	if len(filtered) > 0 {
		return pickMax(filtered).String(), nil
	}

	// Conflict: nothing satisfies all constraints. Return latest overall + warning.
	choice := pickMax(candidates).String()
	cands := make([]string, len(candidates))
	for i, v := range candidates {
		cands[i] = v.String()
	}

	return choice, &ConflictError{
		Constraints: constraints,
		Candidates:  cands,
		Chosen:      choice,
	}
}

// collectCandidates scans constraint strings for version-like tokens and returns
// a deduped slice of parsed *semver.Version. It normalizes partials:
//   "16" -> "16.0.0", "16.2" -> "16.2.0". Leading "v" is allowed.
// Prereleases like "20.0.0-rc.1" are preserved.
// Additionally, it synthesizes implied candidates from comparators:
//   >v  or >=v  -> add (v.major+1).0.0
//   <v  or <=v  -> add (v.major-1).0.0  (if major>0)
func collectCandidates(cons []string) []*semver.Version {
	// plain version literals (no operator required)
	litRe := regexp.MustCompile(`(?i)\bv?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([0-9A-Za-z.-]+))?\b`)
	// operator + version (captures the operator)
	opRe := regexp.MustCompile(`(?i)(?:^|[^\w-])(>=|<=|>|<|=)\s*v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([0-9A-Za-z.-]+))?`)

	seen := make(map[string]struct{})
	out := make([]*semver.Version, 0, 8)

	add := func(s string) {
		v, err := semver.NewVersion(s)
		if err != nil {
			return
		}
		key := v.Original()
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}

	normalize := func(maj, min, pat, pre string) string {
		if min == "" {
			min = "0"
		}
		if pat == "" {
			pat = "0"
		}
		n := maj + "." + min + "." + pat
		if pre != "" {
			n += "-" + pre
		}
		return n
	}

	// 1) Add all explicit literals
	for _, s := range cons {
		for _, m := range litRe.FindAllStringSubmatch(s, -1) {
			add(normalize(m[1], m[2], m[3], m[4]))
		}
	}

	// 2) Add implied candidates from operators
	for _, s := range cons {
		for _, m := range opRe.FindAllStringSubmatch(s, -1) {
			op := strings.TrimSpace(m[1])
			majStr := m[2]
			minStr := m[3]
			patStr := m[4]
			pre := m[5]

			// base version normalized (for completeness)
			base := normalize(majStr, minStr, patStr, pre)
			// parse major to compute next/prev major
			maj, _ := strconv.Atoi(majStr)

			switch op {
			case ">":
				// next major
				add(fmt.Sprintf("%d.0.0", maj+1))
			case "<":
				// previous major if possible
				if maj > 0 {
					add(fmt.Sprintf("%d.0.0", maj-1))
				}
			case "=":
				// nothing extra to add
				_ = base
			}
		}
	}

	return out
}

// collectVersionLiterals scans constraint strings for version-like tokens.
// Supported forms:
//
//	v?MAJOR[.MINOR][.PATCH][-PRERELEASE]
//
// Partials are zero-filled to valid semver (e.g., "16" -> "16.0.0").
// Returns a deduplicated slice of parsed *semver.Version.
func collectVersionLiterals(constraints []string) []*semver.Version {
	// Regex parts:
	//  - optional leading v/V
	//  - major
	//  - optional .minor
	//  - optional .patch
	//  - optional -prerelease (alnum, dot, hyphen)
	re := regexp.MustCompile(`(?i)\bv?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([0-9A-Za-z.-]+))?\b`)

	seen := make(map[string]struct{})
	var out []*semver.Version

	for _, s := range constraints {
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			maj := m[1]
			min := m[2]
			pat := m[3]
			pre := m[4]

			if min == "" {
				min = "0"
			}
			if pat == "" {
				pat = "0"
			}

			b := strings.Builder{}
			b.WriteString(maj)
			b.WriteByte('.')
			b.WriteString(min)
			b.WriteByte('.')
			b.WriteString(pat)
			if pre != "" {
				b.WriteByte('-')
				b.WriteString(pre)
			}
			norm := b.String()

			// Parse to confirm it's a valid semver and normalize to canonical form.
			v, err := semver.NewVersion(norm)
			if err != nil {
				continue
			}
			key := v.Original() // canonicalized by semver lib
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}
