package mkenv

import (
	"context"
	"os"
	"time"

	"github.com/0xa1bed0/mkenv/internal/cache"
	"github.com/0xa1bed0/mkenv/internal/cli"
	"github.com/0xa1bed0/mkenv/internal/dockerclient"
	"github.com/0xa1bed0/mkenv/internal/dockerfile"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
	"github.com/0xa1bed0/mkenv/internal/project"
	"github.com/spf13/cobra"
)

type runOptions struct {
	Tools        []string
	Langs        []string
	Volumes      []string
	ForceRebuild bool
	CleanCache   bool
}

func newRunCmd() *cobra.Command {
	opts := &runOptions{}

	cmd := &cobra.Command{
		Use:   "run [PATH]",
		Short: "Run a dev container for the project",
		Long:  "Build (if needed) and run a dev container for the given project path.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCmdRunE(cmd, args)
		},
	}

	cmd.Flags().StringSliceVar(&opts.Tools, "tools", nil, "Comma-separated tools to preconfigure (e.g. 'mux,nvim')")
	cmd.Flags().StringSliceVar(&opts.Langs, "langs", nil, "Comma-separated languages to enable (e.g. 'nodejs,go')")
	cmd.Flags().StringSliceVarP(&opts.Volumes, "volume", "v", nil, "Bind mount in 'host:container' format (may be repeated)")
	cmd.Flags().BoolVar(&opts.ForceRebuild, "rebuild", false, "Force rebuild of the dev image")
	cmd.Flags().BoolVar(&opts.CleanCache, "clean-cache", false, "Clean build cache before running")

	// Store opts in command context
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		cmd.SetContext(withRunOptions(cmd.Context(), opts))
	}

	return cmd
}

// Separate function so root can reuse it (default command)
func runCmdRunE(cmd *cobra.Command, args []string) error {
	opts := getRunOptions(cmd.Context())
	if opts == nil {
		opts = &runOptions{} // for the rootCmd path (no flags bound)
	}

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

	// TODO; refactor project - maybe introduce sqlite or something like that
	project := project.ResolveProject(pathArg)

	enablebricks := []dockerfile.BrickID{"shell/ohmyzsh"}
	for _, tool := range opts.Tools {
		enablebricks = append(enablebricks, dockerfile.BrickID("tools/"+tool))
	}
	for _, lang := range opts.Langs {
		enablebricks = append(enablebricks, dockerfile.BrickID("langs/"+lang))
	}

	// TODO: read this from filesystem in home dir, read this from project dir, read from args, merge
	userPrefs := &dockerfile.UserPreferences{
		EnableBricks:  enablebricks,
		SystemBrickId: "system/debian",
		BricksConfigs: map[dockerfile.BrickID]map[string]string{},
	}

	// TODO: move this path to constants
	cacheManager, err := cache.NewCacheManager("/Users/anatolii/.mkenv/image-cache.json")
	if err != nil {
		// TODO: ignore this error
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	// TODO: should this be part of project?
	pathPtr, err := filesmanager.NewFileManager(project.Path)
	if err != nil {
		panic(err)
	}

	planner := dockerfile.NewPlanner(pathPtr, userPrefs)
	dockerClient, err := dockerclient.NewDockerClient()
	if err != nil {
		panic(err)
	}

	orchestrator := cli.NewDockerImageBuildOrchestrator(dockerClient, cacheManager, planner)

	imageTag, err := orchestrator.ResolveImageTag(ctx, project.Path, userPrefs, opts.ForceRebuild)
	// TODO: fix it
	project.ImageID = imageTag

	binds := []string{}
	// TODO: figure how to get MKENV_HOME here so we know where to mount these in advance
	binds = append(binds, "/Users/anatolii/projects/albedo/nvim:/home/dev/.config/nvim")
	binds = append(binds, "/Users/anatolii/projects/albedo/tmux.conf:/home/dev/.tmux.conf")
	// TODO: since we can't get var substitution here (replace ${MKENV_WORKDIR}) - lets make single and constant workdir across all envs
	binds = append(binds, project.Path+":/workdir")
	// TODO: binds should also include whatever sits in opts (cli args)

	if err != nil {
		panic(err)
	}

	bgCtx := context.Background()
	exitCode, err := dockerClient.RunContainer(bgCtx, project, binds)
	if err != nil {
		panic(err)
	}
	os.Exit(int(exitCode))
	return nil
}

type ctxKeyRunOptions struct{}

func withRunOptions(ctx context.Context, opts *runOptions) context.Context {
	return context.WithValue(ctx, ctxKeyRunOptions{}, opts)
}

func getRunOptions(ctx context.Context) *runOptions {
	v := ctx.Value(ctxKeyRunOptions{})
	if v == nil {
		return nil
	}
	return v.(*runOptions)
}
