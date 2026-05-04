package gpgclaimtty

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/0xa1bed0/mkenv/internal/networking/sandbox"
	"github.com/spf13/cobra"
)

func NewGPGClaimTTYCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gpg-claim-tty",
		Short: "Reattach host gpg-agent pinentry to this session's host TTY",
		Long: `Sends "gpg-connect-agent updatestartuptty /bye" to the host gpg-agent so that
the next pinentry prompt appears on the host terminal that launched this mkenv
session.

Run this when you've moved between mkenv shells and want pinentry (e.g. for
YubiKey unlock) to surface in the terminal you're currently using, instead of
the one where mkenv was first started.`,
		Args: cobra.NoArgs,
		RunE: runClaimTTY,
	}

	return cmd
}

func runClaimTTY(cmd *cobra.Command, args []string) error {
	controlClient, err := sandbox.NewControlClientFromEnv(cmd.Context())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	resp, err := controlClient.GPGClaimTTY(ctx)
	if err != nil {
		return fmt.Errorf("gpg-claim-tty: %w", err)
	}

	if resp.TTY != "" {
		fmt.Fprintf(os.Stderr, "claimed host TTY: %s\n", resp.TTY)
	}
	if resp.Stdout != "" {
		fmt.Fprint(os.Stdout, resp.Stdout)
	}
	if resp.Stderr != "" {
		fmt.Fprint(os.Stderr, resp.Stderr)
	}

	if resp.ExitCode != 0 {
		return fmt.Errorf("gpg-connect-agent exited with code %d", resp.ExitCode)
	}

	return nil
}
