package mkenv

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mkenv [PATH]",
	Short: "Instant dev containers for your project",
	Long: `mkenv creates secure, isolated dev containers for your project.

By default, 'mkenv' is equivalent to 'mkenv run [PATH]'.
If PATH is omitted, the current working directory is used.`,
	Args: cobra.MaximumNArgs(1),
	// Default behavior is the same as 'run'
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCmdRunE(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Register subcommands here
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newAttachCmd())
	rootCmd.AddCommand(newCleanCmd())
}

