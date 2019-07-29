package webfscmd

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/spf13/cobra"
)

func init() {
	newVol.Flags().String("path", "", "--path=/mypath")
	newVol.Flags().StringP("file", "f", "", "-f")
	newCmd.AddCommand(newVol)
}

var newVol = &cobra.Command{
	Use: "volume",
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
		specFilePath, err := cmd.Flags().GetString("file")
		if err != nil {
			return err
		}
		if specFilePath == "" {
			return errors.New("no filepath provided")
		}

		data, err := ioutil.ReadFile(specFilePath)
		if err != nil {
			return err
		}
		spec := models.CellSpec{}
		if err := json.Unmarshal(data, &spec); err != nil {
			return err
		}

		return wfs.NewVolume(ctx, path, spec)
	},
	Aliases: []string{"vol"},
}
