// mkenv is a CLI tool that inspects a project and emits a ready-to-run
// devcontainer Dockerfile.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xa1bed0/mkenv/internal/dockerimagebuilder"
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

	image, err := dockerimagebuilder.NewImage(pathPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	dockerfile, err := image.Compile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(dockerfile)
	fmt.Println()
}
