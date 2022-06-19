package webfscmd

import (
	"bufio"
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List files and directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			var p string
			if len(args) > 0 {
				p = args[0]
			}
			w := bufio.NewWriter(cmd.OutOrStdout())
			if err := wfs.Ls(ctx, p, func(de fs.DirEntry) error {
				perm := de.Type().Perm()
				_, err := fmt.Fprintf(w, "%v %-20s\n", perm, de.Name())
				return err
			}); err != nil {
				return err
			}
			return w.Flush()
		},
	}
}
