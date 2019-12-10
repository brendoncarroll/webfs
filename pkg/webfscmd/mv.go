package webfscmd

import (
	"errors"
	"log"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(moveCmd)
}

var moveCmd = &cobra.Command{
	Use:   "mv",
	Short: "Copys the object at args[0] to args[1]",

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}
		if len(args) < 2 {
			return errors.New("must specify source and destination")
		}
		if len(args) > 2 {
			return errors.New("webfs mv does not support moving multiple files")
		}

		src, dst := args[0], args[1]
		o, err := wfs.Lookup(ctx, src)
		if err != nil {
			return err
		}

		log.Println("moving", src, "->", dst)
		return wfs.Move(ctx, o, dst)
	},
}
