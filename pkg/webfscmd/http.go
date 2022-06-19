package webfscmd

import (
	"github.com/spf13/cobra"
)

func newHTTPCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "http",
		Short: "serve files over http",
	}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return nil
	}
	return c
}
