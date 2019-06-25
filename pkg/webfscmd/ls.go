package webfscmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(lsCmd)
}

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "ls",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		p := "/"
		if len(args) > 0 {
			p = args[0]
		}
		fmt.Println(p)
		entries, err := wfs.Ls(ctx, p)
		if err != nil {
			return err
		}
		for _, e := range entries {
			fmt.Print(" ")
			fmt.Println(e.Name, e.Object)
		}
		return nil
	},
}
