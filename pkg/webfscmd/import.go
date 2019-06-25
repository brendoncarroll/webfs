package webfscmd

import (
	"errors"
	"log"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(importCmd)
}

var importCmd = &cobra.Command{
	Use: "import",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return errors.New("path and dest required")
		}

		p := args[0]
		dst := args[1]

		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer func() {
			if err := f.Close(); err != nil {
				log.Println(err)
			}
		}()

		return wfs.ImportFile(ctx, f, dst)
	},
}
