package webfscmd

import (
	"errors"
	"fmt"
	"log"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/spf13/cobra"
)

func init() {
	cellCmd.AddCommand(joinCellCmd)

	joinCellCmd.Flags().String("vol", "", "--vol=ABCDEFGH")
}

var joinCellCmd = &cobra.Command{
	Use: "join",
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

		join, ok := cell.(cells.Join)
		if !ok {
			log.Println(cell)
			return errors.New("cell does not support join")
		}

		err = join.Join(ctx, func(x interface{}) bool {
			cmd.Printf("%+v\n", x)
			cmd.Println("OK TO JOIN (true/false)?")

			ok := false
			if _, err := fmt.Fscan(cmd.InOrStdin(), ok); err != nil {
				return false
			}

			return ok
		})
		return err
	},
}
