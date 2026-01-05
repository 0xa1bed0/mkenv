package install

import (
	"context"
	"fmt"
	"time"

	"github.com/0xa1bed0/mkenv/internal/networking/sandbox"
	"github.com/spf13/cobra"
)

func NewInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <pkg>",
		Short: "Install package to the system",
		Long: `You can't use apt install directly since it requires sudo which environment does not have.

mkenv provides a controlled and audited endpoint for additional installation to the environment if needed.
This is not recommened approach to install packages to the system. The best way is to update your .mkenv file and list anything needed there.
Consider checking documentation for further recommenedations.`,
		Args: cobra.ExactArgs(1),
		RunE: runInstall,
	}

	return cmd
}

func runInstall(cmd *cobra.Command, args []string) error {
	pkgName := args[0]

	controlClient, err := sandbox.NewControlClientFromEnv(cmd.Context())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := controlClient.Install(ctx, pkgName)
	if err != nil {
		return fmt.Errorf("install error: %w", err)
	}

	fmt.Print(result.Logs)

	return nil
}
