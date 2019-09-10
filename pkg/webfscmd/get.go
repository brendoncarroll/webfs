package webfscmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

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
		case "vol", "volume":
			return getVol(args[1:])
		case "vols", "volumes":
			return getVols(args[1:])
		default:
			return errors.New("type not recognized")
		}
	},
}

var volHeaders = []string{"ID", "PATH", "URL"}

func getVol(args []string) error {
	id := args[0]
	vol, err := wfs.GetVolume(ctx, id)
	if err != nil {
		return err
	}
	fmt.Println(vol.Describe())
	return nil
}

func getVols(args []string) error {
	vols, err := wfs.ListVolumes(ctx)
	if err != nil {
		return err
	}
	recs := [][]string{volHeaders}
	for _, vol := range vols {
		rec := []string{vol.ID(), vol.Path().String(), vol.URL()}
		recs = append(recs, rec)
	}
	return printTable(os.Stdout, recs)
}

func printTable(w io.Writer, rows [][]string) error {
	bufw := bufio.NewWriter(w)

	colLengths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i := range row {
			l := len(row[i])
			if l > colLengths[i] {
				colLengths[i] = l
			}
		}
	}

	for _, row := range rows {
		for i := range row {
			bufw.WriteString(row[i])
			for c := 0; c < colLengths[i]-len(row[i]); c++ {
				bufw.WriteByte(' ')
			}
			bufw.WriteByte(' ')
		}
		bufw.WriteByte('\n')
	}
	return bufw.Flush()
}
