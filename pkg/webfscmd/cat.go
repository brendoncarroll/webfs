package webfscmd

import (
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(catCmd)
}

var catCmd = &cobra.Command{
	Use: "cat",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 0 {
			return errors.New("missing path")
		}
		p := args[0]
		r, err := wfs.Cat(ctx, p)
		if err != nil {
			return err
		}
		_, err = io.Copy(os.Stdout, r)
		return err
	},
}
