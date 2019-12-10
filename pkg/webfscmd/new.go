package webfscmd

import (
	"log"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

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
		_, err := webfs.NewSuperblock(p)
		if err != nil {
			return err
		}
		log.Println("created superblock at", p)
		return nil
	},
}
