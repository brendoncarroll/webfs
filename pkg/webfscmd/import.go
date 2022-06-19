package webfscmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {

}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import a file or directory into WebFS",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setupWfs(); err != nil {
			return err
		}
		if len(args) < 2 {
			return errors.New("path and dest required")
		}

		p := args[0]
		dst := args[1]

		return importPath(ctx, p, dst)
	},
}

func importPath(ctx context.Context, p, dst string) error {
	fmt.Println("importing", p, "->", dst)
	finfo, err := os.Stat(p)
	if err != nil {
		return err
	}
	if finfo.IsDir() {
		if err := wfs.Mkdir(ctx, dst); err != nil {
			return err
		}
		return importDir(ctx, p, dst)
	}
	return importFile(ctx, p, dst)
}

func importDir(ctx context.Context, p, dst string) error {
	f, err := os.Open(p)
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
		subpath := filepath.Join(p, name)
		subdst := path.Join(dst, name)
		if err := importPath(ctx, subpath, subdst); err != nil {
			return err
		}
	}
	return nil
}

func importFile(ctx context.Context, p, dst string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Println(err)
		}
	}()
	return wfs.ImportFile(ctx, f, dst)
}
