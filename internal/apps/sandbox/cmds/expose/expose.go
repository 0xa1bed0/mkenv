package expose

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/0xa1bed0/mkenv/internal/networking/sandbox"
	"github.com/spf13/cobra"
)

func NewExposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expose <port>",
		Short: "Ask host to expose a port to the host machine",
		Long: `This is unnecesary unless "mkenv sandbox daemon" running. 

But might be useful to claim port in this container and prevent race conditions when your server restarts
This command will claim the port and binds it to the current container, so even when your server stops the port will not be released.
To release binding just exit this command.`,
		Args: cobra.ExactArgs(1),
		RunE: runSandboxExpose,
	}

	return cmd
}

func runSandboxExpose(cmd *cobra.Command, args []string) error {
	port, err := strconv.Atoi(args[0])
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port %q", args[0])
	}

	controlClient, err := sandbox.NewControlClientFromEnv(cmd.Context())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = controlClient.Expose(ctx, port)
	if err != nil {
		return fmt.Errorf("expose error: %w", err)
	}

	fmt.Printf("[mkenv] port %d exposed successfully\nYou can run your server now.", port)
	<-cmd.Context().Done()

	return nil
}
