package webfscmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(rmCmd)
}

var rmCmd = &cobra.Command{
	Short: "Remove an item from a directory",
	Use:   "rm",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}
		if len(args) < 1 {
			return errors.New("must provide path to remove")
		}
		p := args[0]
		err := wfs.Remove(ctx, p)
		return err
	},
}
