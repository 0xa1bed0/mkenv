package runcmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/0xa1bed0/mkenv/internal/agentdist"
	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/dockerclient"
	"github.com/0xa1bed0/mkenv/internal/dockercontainer"
	"github.com/0xa1bed0/mkenv/internal/dockerimage"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/0xa1bed0/mkenv/internal/state"

	"github.com/spf13/cobra"
)

type runOptions struct {
	Tools        []string
	Langs        []string
	Volumes      []string
	Entrypoint   string
	System       string
	Shell        string
	ForceRebuild bool
	CleanCache   bool
}

// AttachRunCmdFlags attaches the "run" cmd flags to the given command and
// injects a runOptions instance into the command's context via PreRun.
func AttachRunCmdFlags(cmd *cobra.Command) {
	opts := &runOptions{}

	flags := cmd.Flags()
	flags.StringSliceVar(&opts.Tools, "tools", nil, "Comma-separated tools to preconfigure (e.g. 'tmux,nvim')")
	flags.StringSliceVar(&opts.Langs, "langs", nil, "Comma-separated languages to enable (e.g. 'nodejs,go')")
	flags.StringVar(&opts.Entrypoint, "entrypoint", "", "Entrypoint brick id (e.g. 'tmux')")
	flags.StringVar(&opts.System, "system", "debian", "System brick id (e.g. 'debian')")
	flags.StringVar(&opts.Shell, "shell", "ohmyzsh", "Shell to enable")
	flags.StringSliceVar(&opts.Volumes, "volume", nil, "Bind mount in 'host:container' format (may be repeated)")
	flags.BoolVar(&opts.ForceRebuild, "build", false, "Force rebuild of the dev image")

	// Store opts in command context before running
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		cmd.SetContext(withRunOptions(cmd.Context(), opts))
	}
}

func (ro *runOptions) EnvConfig() runtime.EnvConfig {
	enableBricks := []bricksengine.BrickID{}

	for _, b := range ro.Tools {
		enableBricks = append(enableBricks, bricksengine.BrickID(b))
	}

	for _, b := range ro.Langs {
		enableBricks = append(enableBricks, bricksengine.BrickID(b))
	}

	if ro.Shell != "" {
		enableBricks = append(enableBricks, bricksengine.BrickID(ro.Shell))
	}

	cliRunConfig := runtime.BuildEnvConfig(
		runtime.WithEnableBricks(enableBricks),
		runtime.WithDefaultEntrypointBrickID(bricksengine.BrickID(ro.Entrypoint)),
		runtime.WithDefaultSystemBrickID(bricksengine.BrickID(ro.System)),
	)

	return cliRunConfig
}

func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [PATH]",
		Short: "Run a dev container for the project",
		Long: `Build (if needed) and run a dev container for the given project path.

If PATH is omitted, the current working directory is used.`,
		Args: cobra.MaximumNArgs(1),
		RunE: RunCmdRunE,
	}

	AttachRunCmdFlags(cmd)

	return cmd
}

// RunCmdRunE is a separate function so root can reuse it (default command)
func RunCmdRunE(cmd *cobra.Command, args []string) error {
	logs.Debugf("running environment...")

	rt := runtime.FromContext(cmd.Context())
	opts := getRunOptions(cmd.Context())
	if opts == nil {
		// This should not normally happen because addRunFlags sets it,
		// but keep a safe fallback for root or tests.
		opts = &runOptions{}
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

	signalsCtx, stopSignalsCtx := signal.NotifyContext(rt.Ctx(), os.Interrupt, syscall.SIGTERM)
	defer stopSignalsCtx()

	kvStore, err := state.DefaultKVStore(signalsCtx)
	if err != nil {
		return err
	}

	project, err := rt.ResolveProject(signalsCtx, pathArg, kvStore)
	if err != nil {
		return err
	}

	project.SetEnvConfigOverride(opts.EnvConfig())

	dockerImageResolver, err := dockerimage.DefaultDockerImageResolver(signalsCtx)
	if err != nil {
		return err
	}

	imageID, err := dockerImageResolver.ResolveImageID(signalsCtx, rt.Project())
	if err != nil {
		return err
	}

	rt.Container().SetImageTag(string(imageID))

	binds, err := mkbinds(signalsCtx, project)
	if err != nil {
		return err
	}

	stopSignalsCtx()

	dockerClient, err := dockerclient.DefaultDockerClient()
	if err != nil {
		return err
	}

	orchestratorExitChan := make(chan dockercontainer.OrchestratorExitSignal, 1)
	containerOrchestrator, err := dockercontainer.NewContainerOrchestrator(rt, binds, dockerClient, orchestratorExitChan)
	if err != nil {
		return err
	}

	return containerOrchestrator.Start()
}

func mkbinds(ctx context.Context, project *runtime.Project) ([]string, error) {
	binds, err := ResolveBinds(project.EnvConfig(ctx).Volumes())
	if err != nil {
		return nil, err
	}

	binds = append(binds, project.Path()+":/workdir")

	agentHostPath := hostappconfig.AgentBinaryPath(project.Name())
	if err := agentdist.ExtractAgent(agentHostPath); err != nil {
		return nil, err
	}
	binds = append(binds, agentHostPath+":"+agentHostPath+":ro")

	return binds, nil
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
