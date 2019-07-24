package webfscmd

import (
	"github.com/brendoncarroll/webfs/pkg/webui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(webuiCmd)
}

var webuiCmd = &cobra.Command{
	Use: "webui",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}
		return webui.ServeUI(wfs)
	},
}
