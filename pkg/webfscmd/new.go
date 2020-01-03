package webfscmd

import (
	"log"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

const defaultSuperblockPath = "superblock.webfs"

func init() {
	newCmd.AddCommand(newFS)
	rootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new instance of any WebFS object",
}

var newFS = &cobra.Command{
	Use: "fs",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := "superblock.webfs"
		if len(args) > 0 {
			p = args[0]
		}
		sb, err := webfs.NewSuperblock(p)
		if err != nil {
			return err
		}
		log.Println("created superblock at", p)
		wfs, err := webfs.New(sb, nil)
		if err != nil {
			return err
		}
		return wfs.Sync(ctx)
	},
}
