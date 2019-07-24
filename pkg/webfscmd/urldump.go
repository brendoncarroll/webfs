package webfscmd

import (
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(urldumpCmd)
}

var urldumpCmd = &cobra.Command{
	Use: "urldump",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}

		err := wfs.RefIter(ctx, func(ref webfs.Ref) bool {
			for _, u := range ref.GetURLs() {
				fmt.Println(u)
			}
			return true
		})
		return err
	},
}
