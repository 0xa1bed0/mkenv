package versions

import (
	"errors"
	"testing"
)

func TestMaxVersion_Conficting(t *testing.T) {
	got, err := MaxVersionFromConstraints([]string{
		">=16.0.0 <17.0.0",
		"^16.14.2",
		">=20",
	})
	if err == nil {
		t.Fatalf("should throw conflicting error")
	}
	if !errors.Is(err, ErrConflictingConstraints) {
		t.Fatalf("expected ConflictError via errors.Is")
	}

	want := "20.0.0"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMaxVersion_ShouldIncrementMaxversionIfRequiredMoreThan(t *testing.T) {
	got, err := MaxVersionFromConstraints([]string{
		">17.0.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "18.0.0"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMaxVersion_BasicRangeAndCaret(t *testing.T) {
	got, err := MaxVersionFromConstraints([]string{
		">=16.0.0 <17.0.0",
		"^16.14.2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "16.14.2"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMaxVersion_ORLogic(t *testing.T) {
	got, err := MaxVersionFromConstraints([]string{
		"<=16.15.0 || >=18 <19",
		">=16.0.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Candidates extracted: 16.15.0, 18.0.0, 19.0.0, 16.0.0
	// The largest satisfying both constraints is 18.0.0
	want := "18.0.0"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMaxVersion_PrereleaseBoundary(t *testing.T) {
	got, err := MaxVersionFromConstraints([]string{
		">=20.0.0-rc.1 <20.0.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "20.0.0-rc.1"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMaxVersion_LeadingVAndPartial(t *testing.T) {
	got, err := MaxVersionFromConstraints([]string{
		"^v16.13.1", // candidate literal “v16.13.1” → 16.13.1
		">=16",      // partial “16” → 16.0.0 (doesn't beat 16.13.1)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "16.13.1"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMaxVersion_DedupAndAndedBounds(t *testing.T) {
	got, err := MaxVersionFromConstraints([]string{
		">=16",
		"<17",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Candidates: 16.0.0 and 17.0.0; only 16.0.0 satisfies both.
	want := "16.0.0"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMaxVersion_NoSatisfyingCandidate(t *testing.T) {
	_, err := MaxVersionFromConstraints([]string{
		"<1.0.0",  // candidate 1.0.0
		">=2.0.0", // candidate 2.0.0
	})
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

func TestMaxVersion_InvalidConstraint(t *testing.T) {
	_, err := MaxVersionFromConstraints([]string{"this-is-not-a-constraint"})
	if err == nil {
		t.Fatal("expected error for invalid constraint, got none")
	}
}

func TestMaxVersion_WildcardsDontCreateNewCandidates(t *testing.T) {
	// Wildcards like "1.x" don’t add candidates beyond explicit numbers present.
	// Here only “1” and “2.5” should be extracted.
	got, err := MaxVersionFromConstraints([]string{
		">=1.x <3", // literals extracted: 1, 3 → 1.0.0 and 3.0.0
		"^2.5.0",   // literal: 2.5.0
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Candidates: 1.0.0, 3.0.0, 2.5.0 → only 2.5.0 satisfies both
	want := "2.5.0"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
