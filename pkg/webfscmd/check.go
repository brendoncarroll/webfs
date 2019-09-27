package webfscmd

import (
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use: "check",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}

		ch := make(chan webfs.RefStatus)
		go func() {
			for x := range ch {
				var sym string
				if x.IsAlive() {
					sym = "✓️️"
				} else {
					sym = "✗"
				}

				fmt.Printf("%s %s\n", sym, x.URL)
				if x.Error != nil {
					fmt.Printf("└─ %s\n", x.Error)
				}
			}
		}()

		return wfs.Check(ctx, ch)
	},
}
