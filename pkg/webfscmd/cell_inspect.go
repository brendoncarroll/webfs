package webfscmd

import (
	"errors"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/spf13/cobra"
)

func init() {
	cellCmd.AddCommand(cellInspect)
	cellInspect.Flags().String("vol", "", "--vol=ABCDEFG")
}

var cellInspect = &cobra.Command{
	Use: "inspect",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}
		if err := cmd.ParseFlags(args); err != nil {
			return err
		}
		volID, err := cmd.Flags().GetString("vol")
		if err != nil {
			return err
		}
		vol, err := wfs.GetVolume(ctx, volID)
		if err != nil {
			return err
		}
		cell := vol.Cell()
		if cell == nil {
			return errors.New("volume has no cell")
		}
		cell2, ok := cell.(cells.Inspect)
		if !ok {
			return errors.New("cannot inspect cell")
		}
		s := cell2.Inspect(ctx)
		cmd.Println(s)
		return nil
	},
}
