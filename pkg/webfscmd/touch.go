package webfscmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(touchCmd)
}

var touchCmd = &cobra.Command{
	Use: "touch",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}
		if len(os.Args) < 1 {
			return errors.New("missing path")
		}
		p := os.Args[0]
		_, err := wfs.Touch(ctx, p)
		return err
	},
}
