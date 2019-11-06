package webfscmd

import (
	"github.com/spf13/cobra"
)

func init() {
	newVol.Flags().String("path", "", "--path=/mypath")
	newCmd.AddCommand(newVol)
}

var newVol = &cobra.Command{
	Use:     "volume",
	Aliases: []string{"vol"},

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}

		if err := cmd.ParseFlags(args); err != nil {
			return err
		}
		path, err := cmd.Flags().GetString("path")
		if err != nil {
			return err
		}

		vol, err := wfs.NewVolume(ctx, path)
		if err != nil {
			return err
		}

		cmd.Println(vol.ID())
		return nil
	},
}
