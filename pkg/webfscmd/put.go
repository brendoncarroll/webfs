package webfscmd

import (
	"bytes"
	"errors"
	"io/ioutil"

	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/golang/protobuf/jsonpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(putCmd)
	putCmd.Flags().StringP("file", "f", "", "-f=./myfile.json")
	putCmd.Flags().StringP("path", "p", "", "--path=/path/to/object")
	putCmd.Flags().StringP("type", "t", "", "--type=volume")
	putCmd.Flags().UintP("index", "i", 0, "--index=2")
}

var putCmd = &cobra.Command{
	Use:  "put",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}

		filePath, err := cmd.Flags().GetString("file")
		if err != nil {
			return err
		}
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return err
		}
		buf := bytes.NewBuffer(data)

		p, err := cmd.Flags().GetString("path")
		if err != nil {
			return err
		}
		ty, err := cmd.Flags().GetString("type")
		if err != nil {
			return err
		}
		index, err := cmd.Flags().GetUint("index")
		if err != nil {
			return err
		}

		o := &webfsim.Object{}
		switch ty {
		case "file":
			x := &webfsim.File{}
			if err := jsonpb.Unmarshal(buf, x); err != nil {
				return err
			}
			o.Value = &webfsim.Object_File{x}
		case "dir":
			x := &webfsim.Dir{}
			if err := jsonpb.Unmarshal(buf, x); err != nil {
				return err
			}
			o.Value = &webfsim.Object_Dir{x}
		case "vol", "volume":
			x := &webfsim.VolumeSpec{}
			if err := jsonpb.Unmarshal(buf, x); err != nil {
				return err
			}
			o.Value = &webfsim.Object_Volume{x}
		default:
			return errors.New("type not recognized")
		}

		return wfs.PutAt(ctx, p, int(index), o)
	},
}
