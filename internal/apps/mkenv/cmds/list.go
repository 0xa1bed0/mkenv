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

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [PATH]",
		Aliases: []string{"ls"},
		Short:   "List dev containers for project.",
		Long:    "List running / known mkenv containers. If PATH is given, filter by that project.",
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
				containers, err = dockerClient.ListAllContainer(signalsCtx, false)
				if err != nil {
					return err
				}
			} else {
				var project *runtime.Project
				project, err = rt.ResolveProject(signalsCtx, pathArg, nil)
				if err != nil {
					return err
				}

				containers, err = dockerClient.ListContainers(signalsCtx, project, false)
				if err != nil {
					return err
				}
			}

			if len(containers) == 0 {
				fmt.Println("No containers found")
				return nil
			}

			colums := []ui.Column{
				{Header: "Project"},
				{Header: "Name"},
				{Header: "State"},
				{Header: "Status"},
				{Header: "Created"},
				{Header: "Command"},
			}

			table := ui.NewTable(colums...)

			for _, container := range containers {
				table.AddRow(container.Project, container.Name, container.State, container.Status, container.Created, container.Command)
			}

			fmt.Println("")
			table.Render(os.Stdout)
			fmt.Println("")
			fmt.Println("Use 'mkenv a [name]' to attach or 'mkenv rm [name]' to remove")

			return nil
		},
	}

	return cmd
}
