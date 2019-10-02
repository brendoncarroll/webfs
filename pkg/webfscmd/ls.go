package webfscmd

import (
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(lsCmd)
}

var lsHeaders = []string{"PATH", "SIZE", "OBJECT"}

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List files and directories",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}

		p := "/"
		if len(args) > 0 {
			p = args[0]
		}
		entries, err := wfs.Ls(ctx, p)
		if err != nil {
			return err
		}

		rows := [][]string{lsHeaders}
		for _, e := range entries {
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
			sizeStr := fmt.Sprintf("%20s", humanize.Bytes(e.Object.Size()))
			row := []string{e.Name, sizeStr, oStr}
			rows = append(rows, row)
		}
		return printTable(cmd.OutOrStdout(), rows)
	},
}
