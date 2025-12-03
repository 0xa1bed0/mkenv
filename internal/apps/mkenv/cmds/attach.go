package mkenv

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type ContainerInfo struct {
	ID     string
	Name   string
	Status string
}

func (c *ContainerInfo) OptionLabel() string {
	return fmt.Sprintf("%s  (%s)  [%s]", c.Name, c.ID, c.Status)
}

func (c *ContainerInfo) OptionID() string {
	return c.ID
}

func newAttachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attach [PATH]",
		Aliases: []string{"a"},
		Short:   "Attach to an existing dev container",
		Long:    "Attach your terminal to a running mkenv container for the given project.",
		Args:    cobra.MaximumNArgs(1),
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

			// TODO: find containers by label (project path hash or similar)
			containers := []ContainerInfo{
				{ID: "abc123", Name: "mkenv-myproj-abc123", Status: "running"},
				{ID: "def456", Name: "mkenv-myproj-def456", Status: "running"},
			}

			if len(containers) == 0 {
				fmt.Println("No running mkenv containers to attach to for", path)
				return nil
			}

			// var target ContainerInfo
			// if len(containers) == 1 {
			// 	target = containers[0]
			// } else {
			// 	var err error
			// 	target, err = ui.SelectOne("Attach to which container?", containers)
			// 	if err != nil {
			// 		return err
			// 	}
			// }

			// fmt.Printf("Attaching to %s (%s)...\n", target.Name, target.ID)

			// TODO: docker attach logic, using your dockerclient
			return nil
		},
	}

	return cmd
}
