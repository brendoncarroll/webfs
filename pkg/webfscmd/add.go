package webfscmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <dst> <src>",
		Short: "adds a file or directory to a WebFS instance",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dst, src := args[0], args[1]
			return importPath(ctx, wfs, dst, src)
		},
	}
}

func importPath(ctx context.Context, wfs *webfs.FS, dst, src string) error {
	fmt.Println("importing", src, "->", dst)
	finfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if finfo.IsDir() {
		if err := wfs.Mkdir(ctx, dst); err != nil {
			return err
		}
		return importDir(ctx, wfs, dst, src)
	}
	return importFile(ctx, wfs, dst, src)
}

func importDir(ctx context.Context, wfs *webfs.FS, dst, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Println(err)
		}
	}()

	names, err := f.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		subsrc := filepath.Join(src, name)
		subdst := path.Join(dst, name)
		if err := importPath(ctx, wfs, subdst, subsrc); err != nil {
			return err
		}
	}
	return nil
}

func importFile(ctx context.Context, wfs *webfs.FS, dst, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Println(err)
		}
	}()
	return wfs.PutFile(ctx, dst, f)
}
