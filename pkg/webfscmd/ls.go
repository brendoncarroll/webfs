package webfscmd

import (
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var lsHeaders = []string{"PATH", "SIZE", "OBJECT"}

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List files and directories",
	RunE: func(cmd *cobra.Command, args []string) error {
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
				objs, err := v.GetAtPath(ctx, nil, nil, -1)
				if err != nil {
					oStr = fmt.Sprint(v, "(broken)", err)
				} else if len(objs) > 1 {
					oStr = fmt.Sprint(v, "->", objs[1])
				} else {
					oStr = fmt.Sprint(v, "->", "empty")
				}
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
