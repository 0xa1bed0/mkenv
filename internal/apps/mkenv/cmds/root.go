package mkenv

import (
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/spf13/cobra"
)

var verbosity int

func Execute(rt *runtime.Runtime) error {
	rootCmd := &cobra.Command{
		Use:   "mkenv [PATH]",
		Short: "Instant dev containers for your project",
		Long: `mkenv creates secure, isolated dev containers for your project.

By default, 'mkenv' is equivalent to 'mkenv run [PATH]'.
If PATH is omitted, the current working directory is used.`,
		Args: cobra.MaximumNArgs(1),
		// Default behavior is the same as 'run'
		RunE: runCmdRunE,

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logs.SetDebugVerbosity(verbosity)
			return nil
		},
		// we will handle that
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "increase verbosity level")

	// Root should accept the same flags as `run`
	attachRunCmdFlags(rootCmd)

	runCmd := newRunCmd()
	rootCmd.AddCommand(runCmd)

	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newAttachCmd())
	rootCmd.AddCommand(newCleanCmd())
	rootCmd.AddCommand(newUITestCmd())

	return rootCmd.ExecuteContext(rt.Ctx())
}
