package webfscmd

import (
	"github.com/spf13/cobra"
)

func newMkDirCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mkdir",
		Short: "Makes a new directory",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := args[0]
			return wfs.Mkdir(ctx, p)
		},
	}
}
