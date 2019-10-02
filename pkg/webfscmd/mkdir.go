package webfscmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(mkdirCmd)
}

var mkdirCmd = &cobra.Command{
	Use:   "mkdir",
	Short: "Makes a new directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}
		if len(args) < 1 {
			return errors.New("must supply dir path")
		}
		p := args[0]
		return wfs.Mkdir(ctx, p)
	},
}
