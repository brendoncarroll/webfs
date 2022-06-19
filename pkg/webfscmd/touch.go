package webfscmd

import (
	"bytes"

	"github.com/spf13/cobra"
)

func newTouchCmd() *cobra.Command {
	c := &cobra.Command{
		Use: "touch",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := args[0]
			err := wfs.PutFile(ctx, p, bytes.NewReader(nil))
			return err
		},
	}
	return c
}
