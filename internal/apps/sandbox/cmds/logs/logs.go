package logs

import (
	"context"
	"fmt"
	"time"

	sandboxnet "github.com/0xa1bed0/mkenv/internal/networking/sandbox"
	"github.com/spf13/cobra"
)

const defaultTailLines = 50

func NewLogsCmd() *cobra.Command {
	var tailLines int
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Display mkenv logs for the current sandbox",
		Long: `Display the mkenv logs for the current sandbox environment.

Logs are fetched from the host daemon via control protocol.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := sandboxnet.NewControlClientFromEnv(ctx)
			if err != nil {
				return fmt.Errorf("connecting to host: %w", err)
			}
			defer client.Close()

			if follow {
				return runFollow(ctx, client, tailLines)
			}
			return runTail(ctx, client, tailLines)
		},
	}

	cmd.Flags().IntVarP(&tailLines, "tail", "n", defaultTailLines, "Number of lines to show")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output in real-time")

	return cmd
}

// runTail fetches the last N lines from the host
func runTail(ctx context.Context, client *sandboxnet.ControlClient, numLines int) error {
	resp, err := client.FetchLogs(ctx, 0, 0) // Fetch all lines first
	if err != nil {
		return fmt.Errorf("fetching logs: %w", err)
	}

	// Take last N lines
	start := 0
	if len(resp.Lines) > numLines {
		start = len(resp.Lines) - numLines
	}

	for _, line := range resp.Lines[start:] {
		fmt.Println(line)
	}

	return nil
}

// runFollow streams logs from the host in real-time
func runFollow(ctx context.Context, client *sandboxnet.ControlClient, initialLines int) error {
	// First, fetch and print the last N lines
	resp, err := client.FetchLogs(ctx, 0, 0)
	if err != nil {
		return fmt.Errorf("fetching logs: %w", err)
	}

	// Print last N lines
	start := 0
	if len(resp.Lines) > initialLines {
		start = len(resp.Lines) - initialLines
	}
	for _, line := range resp.Lines[start:] {
		fmt.Println(line)
	}

	// Track offset for polling
	offset := resp.TotalLines

	// Poll for new lines
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(100 * time.Millisecond):
			resp, err := client.FetchLogs(ctx, offset, 0)
			if err != nil {
				// Connection might be lost, just continue trying
				continue
			}

			for _, line := range resp.Lines {
				fmt.Println(line)
			}

			// Update offset
			offset = resp.TotalLines
		}
	}
}
