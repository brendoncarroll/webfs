package webfscmd

import (
	"errors"

	"github.com/brendoncarroll/webfs/pkg/fuseadapt"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(mountCmd)
}

var mountCmd = &cobra.Command{
	Use:   "mount",
	Short: "Mounts a fuse filesystem",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}
		if len(args) < 1 {
			return errors.New("must provide path")
		}
		path := args[0]
		return fuseadapt.MountAndRun(wfs, path)
	},
}
