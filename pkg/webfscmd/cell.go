package webfscmd

import (
	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cellCmd)
}

var cellCmd = &cobra.Command{
	Use: "cell",
}

func findCell(volID string) (cells.Cell, error) {
	vol, err := wfs.GetVolume(ctx, volID)
	if err != nil {
		return nil, err
	}
	return vol.Cell(), nil
}
