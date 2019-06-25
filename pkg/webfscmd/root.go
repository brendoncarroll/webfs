package webfscmd

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

var (
	wfs *webfs.WebFS
	ctx context.Context
)

var rootCmd = &cobra.Command{
	Short: "webfs",
	Use:   "webfs",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		const superblockPath = "./superblock1.json"
		sb := webfs.NewSuperblock(superblockPath)
		wfs, err = webfs.New(sb)
		if err != nil {
			return err
		}
		ctx = context.Background()
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}
