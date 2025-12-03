package mkenv

import (
	"fmt"
	"time"

	"github.com/0xa1bed0/mkenv/internal/logs"
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

			logs.Banner("UI Demo")
			logs.Infof("Starting UI test")
			logs.Infof("Full log: mkenv-uitest.log")

			logs.Banner("Fake Docker Build")
			logs.Infof("Simulating long build with tail output...")
			logs.Warnf("This is only a demo, nothing is really built")

			// TODO: query Docker for containers labeled 'mkenv=1', maybe by project label
			containers := []*ContainerInfo{
				// fake data for now
				{ID: "abc123", Name: "mkenv-myproj-abc123", Status: "running"},
				{ID: "def456", Name: "mkenv-myproj-def456", Status: "exited"},
				{ID: "def458", Name: "mkenv-myproj-def458", Status: "exited"},
				{ID: "def459", Name: "mkenv-myproj-def459", Status: "exited"},
				{ID: "def455", Name: "mkenv-myproj-def455", Status: "exited"},
			}

			logs.PromptSelectOne("please select one", termui.ToSelectOptions(containers))

			// Start tail for fake "docker build"
			tail := logs.NewTailBox("docker build")

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

			logs.PromptSelectMany("please select many", termui.ToSelectOptions(containers))

			logs.Banner("Build Finished")
			logs.Infof("Fake docker build completed successfully")
			logs.Infof("Check the full log file for all lines")

			return nil
		},
	}

	return cmd
}
