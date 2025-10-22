// Tests in this file exercise version comparison helpers.
package versions

import "testing"

func TestMaxVersion(t *testing.T) {
	t.Parallel()

	input := []string{"1.2.3", "1.10.0", "2.0.1", "2.0.0", "0.9.9"}
	got, err := MaxVersion(input)
	if err != nil {
		t.Fatalf("MaxVersion returned error: %v", err)
	}
	if got != "2.0.1" {
		t.Fatalf("MaxVersion = %q, want %q", got, "2.0.1")
	}
}

func TestMinVersion(t *testing.T) {
	t.Parallel()

	input := []string{"10.0.1", "2.5.6", "2.5.6.1", "0.0.9"}
	got, err := MinVersion(input)
	if err != nil {
		t.Fatalf("MinVersion returned error: %v", err)
	}
	if got != "0.0.9" {
		t.Fatalf("MinVersion = %q, want %q", got, "0.0.9")
	}
}

func TestMaxVersionInvalid(t *testing.T) {
	t.Parallel()

	if _, err := MaxVersion([]string{"1.2.beta"}); err == nil {
		t.Fatal("expected error for invalid version token")
	}
}

func TestMinVersionEmpty(t *testing.T) {
	t.Parallel()

	if _, err := MinVersion(nil); err == nil {
		t.Fatal("expected error when no versions provided")
	}
}
