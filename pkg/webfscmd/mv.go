package webfscmd

import (
	"errors"
	"log"

	"github.com/spf13/cobra"
)

func newMvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mv <superblock> <src> <dst>",
		Short: "Copys the object at args[0] to args[1]",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			log.Println("moving", src, "->", dst)
			return errors.New("mv not yet supported")
		},
	}
}
