package webfscmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(getAtCmd)
}

var getAtCmd = &cobra.Command{
	Use: "getat",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}
		if len(args) < 1 {
			return errors.New("must specify path")
		}
		p := args[0]
		objs, err := wfs.GetAtPath(ctx, p)
		if err != nil {
			return err
		}
		cmd.Println("[")
		for i, o := range objs {
			cmd.Print(o.Describe())
			if i < len(objs)-1 {
				cmd.Print(",")
			}
			cmd.Print("\n")
		}
		cmd.Println("]")
		return nil
	},
}
