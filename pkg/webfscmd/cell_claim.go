package webfscmd

import (
	"errors"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/spf13/cobra"
)

func init() {
	cellCmd.AddCommand(cellClaim)
	cellClaim.Flags().String("vol", "", "--vol=ABCDEFG")
}

var cellClaim = &cobra.Command{
	Use: "claim",
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
		cell, err := findCell(volID)
		if err != nil {
			return err
		}
		cell2, ok := cell.(cells.Claim)
		if !ok {
			return errors.New("cannot inspect cell")
		}
		return cell2.Claim(ctx)
	},
}
