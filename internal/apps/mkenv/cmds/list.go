package mkenv

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [PATH]",
		Short: "List dev containers for this project (or all)",
		Long:  "List running / known mkenv containers. If PATH is given, filter by that project.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var path string
			if len(args) == 1 {
				path = args[0]
			} else {
				pwd, err := os.Getwd()
				if err != nil {
					return err
				}
				path = pwd
			}

			// TODO: query Docker for containers labeled 'mkenv=1', maybe by project label
			containers := []ContainerInfo{
				// fake data for now
				{ID: "abc123", Name: "mkenv-myproj-abc123", Status: "running"},
				{ID: "def456", Name: "mkenv-myproj-def456", Status: "exited"},
				{ID: "def458", Name: "mkenv-myproj-def458", Status: "exited"},
				{ID: "def459", Name: "mkenv-myproj-def459", Status: "exited"},
				{ID: "def455", Name: "mkenv-myproj-def455", Status: "exited"},
			}

			if len(containers) == 0 {
				fmt.Println("No mkenv containers found for", path)
				return nil
			}

			// If you just want to print:
			for _, c := range containers {
				fmt.Printf("%s  %s  (%s)\n", c.ID, c.Name, c.Status)
			}

			// Optional: interactive pick when there are multiple
			if len(containers) > 1 {
				// chosen, err := ui.SelectOne("Select container to inspect:", containers)
				// if err != nil {
				// 	return err
				// }
				// fmt.Printf("\nSelected: %s (%s)\n", chosen.Name, chosen.ID)
				// Could show more details, logs, etc.
			}

			return nil
		},
	}

	return cmd
}
