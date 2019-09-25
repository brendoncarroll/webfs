package webfscmd

import (
	"github.com/brendoncarroll/webfs/pkg/cells/rwacryptocell"
	"github.com/golang/protobuf/jsonpb"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(ccpCmd)

	ccpCmd.AddCommand(ccpKeyGen)
}

var ccpCmd = &cobra.Command{
	Short: "Cryptographic Cell Protocol",
	Use:   "ccp",
}

var ccpKeyGen = &cobra.Command{
	Short: "generate a new key",
	Use:   "key-gen",
	RunE: func(cmd *cobra.Command, args []string) error {
		privEnt, err := rwacryptocell.GenerateEntity()
		if err != nil {
			return err
		}
		m := jsonpb.Marshaler{
			Indent: "  ",
		}
		if err = m.Marshal(cmd.OutOrStdout(), privEnt); err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write([]byte("\n"))
		return err
	},
}
