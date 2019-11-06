package webfscmd

import (
	"errors"
	"io/ioutil"

	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
	"github.com/spf13/cobra"
)

func init() {
	putSpecCmd.Flags().String("vol", "", "--vol=ABCDEF")
	putSpecCmd.Flags().StringP("file", "f", "", "-f")
	cellCmd.AddCommand(putSpecCmd)
}

var putSpecCmd = &cobra.Command{
	Use:   "put",
	Short: "Put a cell spec into a volume, replacing the old spec",

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}

		specPath, err := cmd.Flags().GetString("file")
		if err != nil {
			return err
		}
		spec, err := cellSpecFromPath(specPath)
		if err != nil {
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

		return vol.PutCell(ctx, spec)
	},
}

func cellSpecFromPath(p string) (*webfsim.CellSpec, error) {
	if p == "" {
		return nil, errors.New("no filepath provided")
	}
	data, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	spec := webfsim.CellSpec{}

	if err := webref.Decode(webref.CodecJSON, data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}
