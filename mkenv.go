// mkenv is a CLI tool that inspects a project and emits a ready-to-run
// devcontainer Dockerfile.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

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
		EnableBricks:  []dockerfile.BrickID{"tools/nvim", "tools/tmux", "shell/oh-my-zsh"},
		SystemBrickId: "system/debian",
	}

	planner := dockerfile.NewPlanner(pathPtr, userPrefs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	responseChan := planner.Plan(ctx)
	promptRequestChan := planner.UserPromptsChan()
	promptResponsesChan := planner.UserPromptsResponsesChan()

	for {
		select {
		case prompt, ok := <-promptRequestChan:
			if !ok {
				// planner closed prompt channel; wait for result
				continue
			}
			switch req := prompt.(type) {

			case *dockerfile.Warning:
				fmt.Println("warning:", req.Msg)

			// System / Entrypoint choices come through as requests keyed by your planner
			case *dockerfile.UserInputRequest[dockerfile.BrickID]:
				choice := req.Default
				if choice == "" {
					// no default → pick the first option deterministically
					keys := make([]dockerfile.BrickID, 0, len(req.Options))
					for k := range req.Options {
						keys = append(keys, k)
					}
					// prefer "none" if present (for entrypoint), else the lexicographically smallest key
					if idx := slices.Index(keys, dockerfile.BrickID("none")); idx >= 0 {
						choice = dockerfile.BrickID("none")
					} else {
						slices.SortFunc(keys, func(a, b dockerfile.BrickID) int {
							if a < b {
								return -1
							}
							if a > b {
								return 1
							}
							return 0
						})
						if len(keys) > 0 {
							choice = keys[0]
						}
					}
				}
				promptResponsesChan <- dockerfile.UserInputResponse[any]{Key: req.Key, Reponse: choice}

			default:
				fmt.Printf("info: unhandled prompt %T\n", prompt)
			}

		case res := <-responseChan:
			if res.Err != nil {
				fmt.Println("error:", res.Err)
				return
			}
			fmt.Println("plan produced successfully")
			df := dockerfile.GenerateDockerfile(res.BuildPlan)
			fmt.Println(strings.Join(df, "\n"))
			return

		case <-ctx.Done():
			fmt.Println("timed out:", ctx.Err())
			return
		}
	}
}
