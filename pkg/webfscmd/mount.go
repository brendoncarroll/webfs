package webfscmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newMountCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mount",
		Short: "Mounts a fuse filesystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("fuse not yet supported")
			// if err := setupWfs(); err != nil {
			// 	return err
			// }
			// if len(args) < 1 {
			// 	return errors.New("must provide path")
			// }
			// path := args[0]
			// return fuseadapt.MountAndRun(wfs, path)
		},
	}
}
