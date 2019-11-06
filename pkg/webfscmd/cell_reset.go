package webfscmd

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
)

func init() {
	cellCmd.AddCommand(resetCellCmd)
	resetCellCmd.Flags().String("vol", "", "--vol=ABCDE")
}

var resetCellCmd = &cobra.Command{
	Use:   "reset",
	Short: "Resets the local state for the cell",

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
		reset, ok := cell.(interface{ ResetAuxState(context.Context) error })
		if !ok {
			return errors.New("cannot reset cell")
		}
		return reset.ResetAuxState(ctx)
	},
}
