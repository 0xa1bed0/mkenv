package sandbox

import (
	"github.com/0xa1bed0/mkenv/internal/apps/sandbox/cmds/daemon"
	"github.com/0xa1bed0/mkenv/internal/apps/sandbox/cmds/expose"
	"github.com/0xa1bed0/mkenv/internal/apps/sandbox/cmds/install"
	logscmd "github.com/0xa1bed0/mkenv/internal/apps/sandbox/cmds/logs"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/spf13/cobra"
)

var verbosity int

func newSandboxRootCmd(rt *runtime.Runtime) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "mkenv",
		Short: "mkenv convinience tool for sandbox environment",
		Long: `mkenv creates secure, isolated dev containers for your project.

This means you don't have sudo inside container and can't control it's networking.
This cli provides auditable, secure sandbox escape route for developer convinience.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logs.SetDebugVerbosity(verbosity)

			return nil
		},
		// we will handle that
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "increase verbosity level")

	sandbox := &cobra.Command{
		Use:   "sandbox",
		Short: "Sandbox commands",
	}

	sandbox.AddCommand(daemon.NewDaemonCmd())
	sandbox.AddCommand(expose.NewExposeCmd())
	sandbox.AddCommand(install.NewInstallCmd())
	sandbox.AddCommand(logscmd.NewLogsCmd())

	rootCmd.AddCommand(sandbox)

	return rootCmd
}

func Execute(rt *runtime.Runtime) error {
	rootCmd := newSandboxRootCmd(rt)

	return rootCmd.ExecuteContext(rt.Ctx())
}
