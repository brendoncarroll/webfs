package webfscmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use:  "get",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}

		ty := args[0]
		switch ty {
		case "vols", "vol", "volumes", "volume":
			return getVols(args[1:])
		default:
			return errors.New("type not recognized")
		}
	},
}

func getVols(args []string) error {
	vols, err := wfs.ListVolumes(ctx)
	if err != nil {
		return err
	}
	for _, vol := range vols {
		fmt.Println(vol)
	}
	return nil
}
