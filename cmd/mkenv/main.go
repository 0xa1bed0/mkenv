package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0xa1bed0/mkenv/internal/cache"
	"github.com/0xa1bed0/mkenv/internal/cli"
	"github.com/0xa1bed0/mkenv/internal/dockerclient"
	_ "github.com/0xa1bed0/mkenv/internal/registry" // blank import triggers init() in internal/bricks/...

	"github.com/0xa1bed0/mkenv/internal/dockerfile"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

// Config holds CLI options parsed from flags and arguments.
type Config struct {
	Path   string
	Editor string
	Lang   string
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil // exists (file or dir)
	}
	if os.IsNotExist(err) {
		return false, nil // does not exist
	}
	return false, err // some other error (e.g. permission denied)
}

func main() {
	var cfg Config

	// Define flags
	flag.StringVar(&cfg.Editor, "editor", "", "Preferred editor (e.g. nvim, code, vim). Default: none")

	// Override default usage message
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage:\n  %s <path> [--editor=nvim] [--lang=go]\n\nFlags:\n",
			filepath.Base(os.Args[0]),
		)
		flag.PrintDefaults()
	}

	// Parse command-line flags
	flag.Parse()

	// Validate and extract positional arguments
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: missing path argument")
		flag.Usage()
		os.Exit(1)
	}

	cfg.Path = args[0]
	if cfg.Path == "" {
		fmt.Fprintln(os.Stderr, "Error: path cannot be empty")
		os.Exit(1)
	}

	// Normalize to absolute path
	abs, err := filepath.Abs(cfg.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid path: %v\n", err)
		os.Exit(1)
	}
	pathExists, err := pathExists(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !pathExists {
		fmt.Fprintf(os.Stderr, "Error: path %s does not exists\n", abs)
		os.Exit(1)
	}
	cfg.Path = abs

	pathPtr, err := filesmanager.NewFileManager(cfg.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	userPrefs := &dockerfile.UserPreferences{
		EnableBricks:  []dockerfile.BrickID{"tools/nvim", "tools/tmux", "shell/ohmyzsh"},
		SystemBrickId: "system/debian",
		BricksConfigs: map[dockerfile.BrickID]map[string]string{
			"system/debian": {
				"workdir": "${MKENV_HOME}/workspace",
			},
		},
	}

	cacheManager, err := cache.NewCacheManager("/Users/anatolii/.mkenv/image-cache.json")
	if err != nil {
		// TODO: ignore this error
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	planner := dockerfile.NewPlanner(pathPtr, userPrefs)
	dockerClient, err := dockerclient.NewDockerClient()
	if err != nil {
		panic(err)
	}

	orchestrator := cli.NewDockerImageBuildOrchestrator(dockerClient, cacheManager, planner)

	tag, err := orchestrator.ResolveImageTag(ctx, cfg.Path, userPrefs)
	if err != nil {
		panic(err)
	}

	bgCtx := context.Background()
	exitCode, err := dockerClient.RunContainer(bgCtx, tag, []string{
		"/Users/anatolii/projects/albedo/nvim:/home/dev/.config/nvim",
		"/Users/anatolii/projects/albedo/tmux.conf:/home/dev/.tmux.conf",
		cfg.Path+":/home/dev/workspace",
	})
	if err != nil {
		panic(err)
	}
	os.Exit(int(exitCode))
}
