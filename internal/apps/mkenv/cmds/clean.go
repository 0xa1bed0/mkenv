package mkenv

import (
	"fmt"

	"github.com/spf13/cobra"
)

type cleanOptions struct {
	Containers bool
	Images     bool
	Volumes    bool
	Cache      bool
	All        bool
}

func newCleanCmd() *cobra.Command {
	opts := &cleanOptions{}

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up mkenv containers, images, volumes, and cache",
		Long: `Clean up mkenv artifacts.

By default, '--all' is implied, which cleans containers, images, volumes, and cache.
Use flags to be more granular.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no specific flags and !All explicitly set, treat as All
			if !opts.Containers && !opts.Images && !opts.Volumes && !opts.Cache && !opts.All {
				opts.All = true
			}

			if opts.All {
				opts.Containers = true
				opts.Images = true
				opts.Volumes = true
				opts.Cache = true
			}

			fmt.Println("Cleaning:")
			fmt.Printf("  containers: %v\n", opts.Containers)
			fmt.Printf("  images:     %v\n", opts.Images)
			fmt.Printf("  volumes:    %v\n", opts.Volumes)
			fmt.Printf("  cache:      %v\n", opts.Cache)

			// TODO:
			// - docker stop/remove containers with mkenv labels
			// - docker rmi images with mkenv labels
			// - docker volume rm volumes with mkenv labels
			// - delete local cache file/dir

			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.All, "all", false, "Clean everything (default behavior)")
	cmd.Flags().BoolVar(&opts.Containers, "containers", false, "Clean containers only")
	cmd.Flags().BoolVar(&opts.Images, "images", false, "Clean images")
	cmd.Flags().BoolVar(&opts.Volumes, "volumes", false, "Clean volumes")
	cmd.Flags().BoolVar(&opts.Cache, "cache", false, "Clean cache")

	return cmd
}
