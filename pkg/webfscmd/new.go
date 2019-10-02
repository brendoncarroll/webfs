package webfscmd

import (
	"log"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

const superblockPath = "./superblock.json"

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
		_, err := webfs.NewSuperblock(superblockPath)
		if err != nil {
			return err
		}
		log.Println("created superblock at", superblockPath)
		return nil
	},
}
