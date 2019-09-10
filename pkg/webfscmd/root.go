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
	Short: "WebFS",
	Use:   "webfs",
}

func Execute() error {
	return rootCmd.Execute()
}

func setupWfs() (err error) {
	sb := webfs.SuperblockFromPath(superblockPath)
	wfs, err = webfs.New(sb, nil)
	if err != nil {
		return err
	}
	ctx = context.Background()
	return nil
}
