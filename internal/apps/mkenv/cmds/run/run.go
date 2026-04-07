package runcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/0xa1bed0/mkenv/internal/agentdist"
	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	sandboxappconfig "github.com/0xa1bed0/mkenv/internal/apps/sandbox/config"
	"github.com/0xa1bed0/mkenv/internal/bricks/tools"
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/dockerclient"
	"github.com/0xa1bed0/mkenv/internal/dockercontainer"
	"github.com/0xa1bed0/mkenv/internal/dockerimage"
	"github.com/0xa1bed0/mkenv/internal/guardrails"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
	"github.com/0xa1bed0/mkenv/internal/state"

	"github.com/spf13/cobra"
)

type runOptions struct {
	Tools           []string
	Langs           []string
	Volumes         []string
	Entrypoint      string
	System          string
	Shell           string
	ForceRebuild    bool
	CleanCache      bool
	AddDockerSocket bool
	MountGPG        bool
	Command         string
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
	flags.BoolVar(&opts.ForceRebuild, "rebuild", false, "Force rebuild of the dev image. Update image cache for the next runs")
	flags.BoolVar(&opts.AddDockerSocket, "add-docker-socket-i-know-what-i-do", false, "Mount the host Docker socket into the container and install Docker CLI")
	flags.BoolVar(&opts.MountGPG, "mount-gpg", false, "Mount host GPG agent sockets into the container (for YubiKey/smartcard SSH)")
	flags.StringVarP(&opts.Command, "command", "c", "", "Run a single command inside the environment and exit (non-interactive)")

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

	if ro.AddDockerSocket {
		enableBricks = append(enableBricks, "docker-cli")
	}

	if ro.MountGPG {
		enableBricks = append(enableBricks, "gpg-agent")
	}

	cliRunConfig := runtime.BuildEnvConfig(
		runtime.WithEnableBricks(enableBricks),
		runtime.WithDefaultEntrypointBrickID(bricksengine.BrickID(ro.Entrypoint)),
		runtime.WithDefaultSystemBrickID(bricksengine.BrickID(ro.System)),
		runtime.WithVolumes(ro.Volumes),
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

	policy, err := guardrails.LoadPolicy()
	if err != nil {
		return err
	}
	if opts.AddDockerSocket && !policy.AllowDockerSocketMountingBypass() {
		policyPath := filepath.Join(hostappconfig.ConfigBasePath(), "policy.json")
		return fmt.Errorf(
			"--add-docker-socket-i-know-what-i-do is disabled by policy\n\n"+
				"To enable it, create %s (chmod 0444) with:\n"+
				"  {\"allow_docker_socket_mounting_bypass\": true}",
			policyPath,
		)
	}

	dockerImageResolver, err := dockerimage.DefaultDockerImageResolver(signalsCtx)
	if err != nil {
		return err
	}

	imageID, err := dockerImageResolver.ResolveImageID(signalsCtx, rt.Project(), opts.ForceRebuild, policy.ImageMaxAge())
	if err != nil {
		return err
	}

	rt.Container().SetImageTag(string(imageID))

	binds, groupAdd, err := mkbinds(signalsCtx, rt, project, opts.AddDockerSocket, opts.MountGPG)
	if err != nil {
		return err
	}

	stopSignalsCtx()

	dockerClient, err := dockerclient.DefaultDockerClient()
	if err != nil {
		return err
	}

	orchestratorExitChan := make(chan dockercontainer.OrchestratorExitSignal, 1)
	containerOrchestrator, err := dockercontainer.NewContainerOrchestrator(rt, binds, groupAdd, dockerClient, orchestratorExitChan, opts.Command)
	if err != nil {
		return err
	}

	return containerOrchestrator.Start()
}

func mkbinds(ctx context.Context, rt *runtime.Runtime, project *runtime.Project, addDockerSocket bool, mountGPG bool) ([]string, []string, error) {
	binds, err := ResolveBinds(project.EnvConfig(ctx).Volumes())
	if err != nil {
		return nil, nil, err
	}

	binds = append(binds, project.Path()+":/"+filepath.Base(project.Path()))
	// this is a hack for docker desktop race condition on folders on host
	// TODO: invesatigate and fix
	binds = append(binds, hostappconfig.ProjectDataPath(project.Name())+":/mnthack:ro")

	agentHostPath := hostappconfig.AgentBinaryPath(project.Name())
	if err := agentdist.ExtractAgent(agentHostPath); err != nil {
		return nil, nil, err
	}
	//binds = append(binds, agentHostPath+":"+agentHostPath+":ro")
	binds = append(binds, agentHostPath+"/mkenv:"+sandboxappconfig.UserLocalBin+"/mkenv:ro")

	// Mount the host run log file for mkenv sandbox logs command
	hostLogPath := hostappconfig.RunLogPath(project.Name(), rt.RunID())
	binds = append(binds, hostLogPath+":"+sandboxappconfig.HostRunLogFile+":ro")

	var groupAdd []string
	if addDockerSocket {
		socketPath, err := detectDockerSocketPath()
		if err != nil {
			return nil, nil, err
		}
		binds = append(binds, socketPath+":/var/run/docker.sock")

		// Detect the socket's owning GID so the container user can access it.
		fi, err := os.Stat(socketPath)
		if err != nil {
			return nil, nil, fmt.Errorf("stat docker socket: %w", err)
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, nil, fmt.Errorf("cannot determine docker socket GID")
		}
		// Always include GID 0 (root group) to handle nested Docker environments
		// where the socket appears as root:root inside the container regardless of
		// what os.Stat reports from the outer container's GID mapping.
		statGID := fmt.Sprintf("%d", st.Gid)
		if statGID == "0" {
			groupAdd = []string{"0"}
		} else {
			groupAdd = []string{"0", statGID}
		}
	}

	if mountGPG {
		gpgSocketPath, err := detectGPGAgentSocketPath(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("GPG agent socket not found: %w\nEnsure gpg-agent is running (gpgconf --launch gpg-agent)", err)
		}
		binds = append(binds, gpgSocketPath+":"+tools.GPGAgentSocketDir+"/S.gpg-agent")

		// Detect the socket's owning GID so the container user can access it.
		fi, err := os.Stat(gpgSocketPath)
		if err != nil {
			return nil, nil, fmt.Errorf("stat GPG agent socket: %w", err)
		}
		if st, ok := fi.Sys().(*syscall.Stat_t); ok {
			gid := fmt.Sprintf("%d", st.Gid)
			if gid == "0" {
				groupAdd = appendUnique(groupAdd, "0")
			} else {
				groupAdd = appendUnique(groupAdd, "0", gid)
			}
		}

		sshSocketPath, err := detectGPGSSHSocketPath(ctx)
		if err != nil {
			logs.Warnf("GPG SSH socket not found, SSH via GPG agent will not work: %v", err)
		} else {
			binds = append(binds, sshSocketPath+":"+tools.GPGAgentSocketDir+"/S.gpg-agent.ssh")
		}
	}

	return binds, groupAdd, nil
}

func detectDockerSocketPath() (string, error) {
	// 1. Check DOCKER_HOST env var
	if dockerHost := os.Getenv("DOCKER_HOST"); dockerHost != "" {
		return strings.TrimPrefix(dockerHost, "unix://"), nil
	}

	// 2. Try the default Linux/macOS path
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return "/var/run/docker.sock", nil
	}

	// 3. Fallback for macOS Docker Desktop without symlink
	home, err := os.UserHomeDir()
	if err == nil {
		macOSPath := home + "/.docker/run/docker.sock"
		if _, err := os.Stat(macOSPath); err == nil {
			return macOSPath, nil
		}
	}

	return "", fmt.Errorf("could not detect Docker socket path: set DOCKER_HOST or ensure /var/run/docker.sock exists")
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

func detectGPGAgentSocketPath(ctx context.Context) (string, error) {
	// Prefer the main socket — the extra socket restricts pinentry behavior
	// (e.g. broken input, missing key icon) which breaks interactive PIN entry.
	for _, dir := range []string{"agent-socket", "agent-extra-socket"} {
		if p := gpgconfListDir(ctx, dir); p != "" {
			return p, nil
		}
	}

	// Fallback to well-known paths.
	home, _ := os.UserHomeDir()
	gnupgHome := os.Getenv("GNUPGHOME")
	if gnupgHome == "" && home != "" {
		gnupgHome = filepath.Join(home, ".gnupg")
	}
	if gnupgHome != "" {
		for _, name := range []string{"S.gpg-agent", "S.gpg-agent.extra"} {
			p := filepath.Join(gnupgHome, name)
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	return "", fmt.Errorf("could not detect GPG agent socket: ensure gpg-agent is running")
}

func detectGPGSSHSocketPath(ctx context.Context) (string, error) {
	if p := gpgconfListDir(ctx, "agent-ssh-socket"); p != "" {
		return p, nil
	}

	home, _ := os.UserHomeDir()
	gnupgHome := os.Getenv("GNUPGHOME")
	if gnupgHome == "" && home != "" {
		gnupgHome = filepath.Join(home, ".gnupg")
	}
	if gnupgHome != "" {
		p := filepath.Join(gnupgHome, "S.gpg-agent.ssh")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("could not detect GPG SSH socket")
}

// appendUnique appends values to a slice, skipping duplicates.
func appendUnique(slice []string, vals ...string) []string {
	for _, v := range vals {
		found := false
		for _, s := range slice {
			if s == v {
				found = true
				break
			}
		}
		if !found {
			slice = append(slice, v)
		}
	}
	return slice
}

// gpgconfListDir runs "gpgconf --list-dirs <key>" and returns the trimmed
// output if the resulting path exists on disk.
func gpgconfListDir(ctx context.Context, key string) string {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cmdCtx, "gpgconf", "--list-dirs", key).Output()
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return ""
	}
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}
