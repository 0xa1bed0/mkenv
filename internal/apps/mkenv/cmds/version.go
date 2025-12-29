package mkenv

import (
	"fmt"

	"github.com/0xa1bed0/mkenv/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of mkenv",
		Long:  `Display the current version of mkenv.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s\n", version.Get())
		},
	}

	return cmd
}
