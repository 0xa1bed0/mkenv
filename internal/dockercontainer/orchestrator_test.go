package dockercontainer

import (
	"strings"
	"testing"
)

func TestRejectCustomEnvCollisions(t *testing.T) {
	mkenv := []string{
		"MKENV_RPC=tcp",
		"MKENV_ADDR=host.docker.internal:1234",
		"MKENV_REVERSE_PROXY=host.docker.internal:5678",
		"TZ=Etc/UTC",
	}

	t.Run("no collision passes", func(t *testing.T) {
		err := rejectCustomEnvCollisions(mkenv, []string{
			"API_TOKEN=abc",
			"DATABASE_URL=postgres://x",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("TZ collision is rejected", func(t *testing.T) {
		// This is the exact bug fix the user flagged: TZ has no MKENV_ prefix,
		// so the name-blocklist in ResolveCustomEnvs does not catch it. The
		// collision guard MUST.
		err := rejectCustomEnvCollisions(mkenv, []string{"TZ=America/Los_Angeles"})
		if err == nil {
			t.Fatal("expected collision error for TZ, got nil")
		}
		if !strings.Contains(err.Error(), `"TZ"`) {
			t.Fatalf("expected error to name TZ, got %q", err.Error())
		}
	})

	t.Run("control-channel collision is rejected", func(t *testing.T) {
		// Even though MKENV_RPC is also caught earlier by the reserved-prefix
		// check in the resolver, the collision guard is the last line of
		// defense and must catch it independently.
		err := rejectCustomEnvCollisions(mkenv, []string{"MKENV_RPC=hijacked"})
		if err == nil {
			t.Fatal("expected collision error for MKENV_RPC, got nil")
		}
	})

	t.Run("mixed custom envs: collision short-circuits", func(t *testing.T) {
		err := rejectCustomEnvCollisions(mkenv, []string{
			"GOOD=ok",
			"TZ=other",
			"ALSO_GOOD=ok",
		})
		if err == nil {
			t.Fatal("expected collision error, got nil")
		}
	})

	t.Run("malformed entries are ignored", func(t *testing.T) {
		// Defensive — neither resolver nor docker would produce these, but the
		// helper must not panic on them.
		err := rejectCustomEnvCollisions(
			[]string{"=novalue", "MKENV_OK=1"},
			[]string{"=novalue", "OK=1"},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
