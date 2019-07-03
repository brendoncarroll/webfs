package webfscmd

import (
	"log"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

const superblockPath = "./superblock.json"

func init() {
	rootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use: "new",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := webfs.NewSuperblock(superblockPath)
		if err != nil {
			return err
		}
		log.Println("created superblock at", superblockPath)
		return nil
	},
}
