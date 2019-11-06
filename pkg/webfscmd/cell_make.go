package webfscmd

import (
	"crypto/rand"
	"errors"

	"github.com/brendoncarroll/webfs/pkg/cells/rwacryptocell"

	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/golang/protobuf/jsonpb"
	"github.com/spf13/cobra"
)

func init() {
	cellCmd.AddCommand(makeCellCmd)
}

var makeCellCmd = &cobra.Command{
	Use:   "make",
	Short: "Creates a cell spec, serializes it, and writes it to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}
		if len(args) < 1 {
			return errors.New("must specify type")
		}
		return makeCell(cmd, args[0])
	},
}

func makeCell(cmd *cobra.Command, ty string) error {
	var spec *webfsim.CellSpec
	switch ty {
	case "http":
		spec = &webfsim.CellSpec{
			Spec: &webfsim.CellSpec_Http{
				Http: &webfsim.HTTPCellSpec{},
			},
		}

	case "secretbox":
		spec = &webfsim.CellSpec{
			Spec: &webfsim.CellSpec_Secretbox{
				Secretbox: &webfsim.SecretBoxCellSpec{
					Secret: generateSecret(),
				},
			},
		}

	case "rwacrypto":
		priv, err := rwacryptocell.GenerateEntity()
		if err != nil {
			return err
		}
		pub := rwacryptocell.GetPublicEntity(priv)
		spec = &webfsim.CellSpec{
			Spec: &webfsim.CellSpec_Rwacrypto{
				Rwacrypto: &webfsim.RWACryptoCellSpec{
					PrivateEntity: priv,
					PublicEntity:  pub,
				},
			},
		}

	default:
		return errors.New("unrecognized type: " + ty)
	}

	m := jsonpb.Marshaler{
		Indent:       " ",
		EmitDefaults: true,
	}
	s, err := m.MarshalToString(spec)
	if err != nil {
		return err
	}
	cmd.Println(s)
	return nil
}

func generateSecret() []byte {
	n := 32
	buf := make([]byte, n)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return buf
}
