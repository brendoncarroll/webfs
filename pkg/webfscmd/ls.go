package webfscmd

import (
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(lsCmd)
}

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "ls",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}

		p := "/"
		if len(args) > 0 {
			p = args[0]
		}
		fmt.Println(p)
		entries, err := wfs.Ls(ctx, p)
		if err != nil {
			return err
		}
		for _, e := range entries {
			fmt.Print(" ")
			oStr := ""
			if v, ok := e.Object.(*webfs.Volume); ok {
				o, err := v.Lookup(ctx, nil)
				if err != nil {
					return err
				}
				oStr = fmt.Sprint(v, " -> ", o)
			} else {
				oStr = fmt.Sprint(e.Object)
			}

			fmt.Printf("%-20s %10dB %-30s\n", e.Name, e.Object.Size(), oStr)
		}
		return nil
	},
}
