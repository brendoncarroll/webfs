package webfscmd

import (
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Short: "Remove an item from a directory",
		Use:   "rm",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := args[0]
			return wfs.Remove(ctx, p)
		},
	}
}
