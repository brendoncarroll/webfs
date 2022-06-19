package webfscmd

import (
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newCatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cat",
		Short: "Write the contents of a file to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := setupWfs(); err != nil {
				return err
			}
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
}
