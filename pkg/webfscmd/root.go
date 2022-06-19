package webfscmd

import (
	"context"
	"errors"
	"io/ioutil"
	"path/filepath"

	"github.com/brendoncarroll/go-state/posixfs"
	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

func Execute() error {
	rc := NewRootCmd()
	return rc.Execute()
}

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Short: "WebFS",
		Use:   "webfs",
	}
	rootPath := rootCmd.PersistentFlags().StringP("root", "r", "", "-r root.webfs")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if *rootPath == "" {
			return errors.New("must provide a root")
		}
		data, err := ioutil.ReadFile(*rootPath)
		if err != nil {
			return err
		}
		vs, err := webfs.ParseVolumeSpec(data)
		if err != nil {
			return err
		}
		fsRoot, err := filepath.Abs(".")
		if err != nil {
			return err
		}
		opts := []webfs.Option{
			webfs.WithPosixFS(posixfs.NewDirFS(fsRoot)),
		}
		wfs, err = webfs.New(*vs, opts...)
		return err
	}

	for _, c := range []*cobra.Command{
		newCatCmd(),
		newHTTPCmd(),
		newEditCmd(),
		newAddCmd(),
		newLsCmd(),
		newMkDirCmd(),
		newRmCmd(),
		newTouchCmd(),
		newMountCmd(),
		newMvCmd(),
	} {
		rootCmd.AddCommand(c)
	}
	return rootCmd
}

var (
	ctx = context.Background()
	wfs *webfs.FS
)
