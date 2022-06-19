package webfscmd

import (
	"github.com/spf13/cobra"
)

func Execute() error {
	rc := NewRootCmd()
	return rc.Execute()
}

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Short: "WebFS",
		Use:   "webfs",
	}
	for _, c := range []*cobra.Command{
		newCatCmd(),
		newHTTPCmd(),
		newImportCmd(),
		newLsCmd(),
		newMkDirCmd(),
		newRmCmd(),
		newTouchCmd(),
	} {
		rootCmd.AddCommand(c)
	}
	return rootCmd
}
