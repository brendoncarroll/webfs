package webfscmd

import "github.com/spf13/cobra"

func init() {
	rootCmd.AddCommand(touchCmd)
}

var touchCmd = &cobra.Command{
	Use: "touch",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}
