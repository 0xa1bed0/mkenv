package mkenv

import (
	"fmt"
	"os"
	"time"

	termui "github.com/0xa1bed0/mkenv/internal/ui"
	"github.com/spf13/cobra"
)

func newUITestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uitest",
		Short: "Demo the fancy logging + tail UI",
		Long: `Run a fake build that exercises the logging UI:

- prints info/warn logs
- streams a long "docker build" tail
- shows only the last 5 tail lines in a box
- mirrors everything into mkenv-uitest.log`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logFile, err := os.Create("mkenv-uitest.log")
			if err != nil {
				return fmt.Errorf("create log file: %w", err)
			}
			defer logFile.Close()

			ui := termui.New(termui.Options{
				TailLines:     5,
				EnableTail:    true,
				FullLogWriter: logFile,
			})
			defer ui.Close()

			ui.Banner("UI Demo")
			ui.Info("Starting UI test")
			ui.Info("Full log: mkenv-uitest.log")

			ui.Banner("Fake Docker Build")
			ui.Info("Simulating long build with tail output...")
			ui.Warn("This is only a demo, nothing is really built")

			// TODO: query Docker for containers labeled 'mkenv=1', maybe by project label
			containers := []*ContainerInfo{
				// fake data for now
				{ID: "abc123", Name: "mkenv-myproj-abc123", Status: "running"},
				{ID: "def456", Name: "mkenv-myproj-def456", Status: "exited"},
				{ID: "def458", Name: "mkenv-myproj-def458", Status: "exited"},
				{ID: "def459", Name: "mkenv-myproj-def459", Status: "exited"},
				{ID: "def455", Name: "mkenv-myproj-def455", Status: "exited"},
			}

			ui.SelectOne("please select one", termui.ToSelectOptions(containers))

			// Start tail for fake "docker build"
			tail := ui.NewTail("docker build")

			steps := 40
			for i := 1; i <= steps; i++ {
				// Noisy tail output
				tail.Println(fmt.Sprintf("step %02d/%02d: running `RUN some-very-long-command --with --a-lot --of --flags`", i, steps))
				tail.Println(fmt.Sprintf(" -> downloading dependency chunk %d...", i))
				if i%5 == 0 {
					tail.Println(" -> cache miss, rebuilding from source…")
				}
				if i%8 == 0 {
					tail.Println(" -> warning: deprecated API used, consider upgrading base image")
				}

				time.Sleep(150 * time.Millisecond)
			}
			tail.Close()

			ui.SelectMany("please select many", termui.ToSelectOptions(containers))

			ui.Banner("Build Finished")
			ui.Info("Fake docker build completed successfully")
			ui.Info("Check the full log file for all lines")

			return nil
		},
	}

	return cmd
}
