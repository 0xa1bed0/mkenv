package mkenv

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/0xa1bed0/mkenv/internal/dockerclient"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/0xa1bed0/mkenv/internal/ui"
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
			logs.Debugf("running list...")

			rt := runtime.FromContext(cmd.Context())

			pathArg := "."
			if len(args) == 1 {
				pathArg = args[0]
			} else {
				pwd, err := os.Getwd()
				if err != nil {
					return err
				}
				pathArg = pwd
			}

			signalsCtx, stopSignalsCtx := signal.NotifyContext(rt.Ctx(), os.Interrupt, syscall.SIGTERM)
			defer stopSignalsCtx()

			dockerClient, err := dockerclient.DefaultDockerClient()
			if err != nil {
				return err
			}

			var containers []*dockerclient.MkenvContainerInfo

			if len(args) == 0 {
				containers, err = dockerClient.ListAllContainer(signalsCtx, true)
				if err != nil {
					return err
				}
			} else {
				var project *runtime.Project
				project, err = rt.ResolveProject(signalsCtx, pathArg, nil)
				if err != nil {
					return err
				}
				containers, err = dockerClient.ListContainers(signalsCtx, project, true)
				if err != nil {
					return err
				}
			}

			if len(containers) == 0 {
				fmt.Println("No containers found")
				return nil
			}

			if len(containers) == 1 {
				return dockerClient.AttachToRunning(rt.Ctx(), containers[0].ContainerID, containers[0].Project, rt.Term())
			}

			selected, err := logs.PromptSelectOne("Select container to attach to", ui.ToSelectOptions(containers))
			if err != nil {
				return err
			}

			// Find the selected container to get its project name
			var displayName string
			for _, c := range containers {
				if c.ContainerID == selected.OptionID() {
					displayName = c.Project
					break
				}
			}

			return dockerClient.AttachToRunning(rt.Ctx(), selected.OptionID(), displayName, rt.Term())
		},
	}

	return cmd
}
