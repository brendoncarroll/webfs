package webfscmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(delCmd)

	delCmd.Flags().StringP("path", "p", "", "--path=/path/to/object")
	delCmd.Flags().UintP("index", "i", 0, "--index=2")
}

var delCmd = &cobra.Command{
	Short: "delete",
	Use:   "del",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}

		p, err := cmd.Flags().GetString("path")
		if err != nil {
			return err
		}
		index, err := cmd.Flags().GetUint("index")
		if err != nil {
			return err
		}

		if err := wfs.DeleteAt(ctx, p, int(index)); err != nil {
			return err
		}
		return nil
	},
}
