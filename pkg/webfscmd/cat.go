package webfscmd

import (
	"os"

	"github.com/spf13/cobra"
)

func newCatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cat <superblock> <path>",
		Short: "Write the contents of a file to stdout",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := args[0]
			return wfs.Cat(ctx, p, os.Stdout)
		},
	}
}
