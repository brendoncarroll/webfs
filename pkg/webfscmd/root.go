package webfscmd

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/spf13/cobra"
)

var (
	wfs *webfs.WebFS
	ctx context.Context
)

var rootCmd = &cobra.Command{
	Short: "WebFS",
	Use:   "webfs",
}

func Execute() error {
	return rootCmd.Execute()
}

func setupWfs() (err error) {
	sbs, err := listSuperblocks()
	if err != nil {
		return err
	}
	var superblockPath string
	switch {
	case len(sbs) > 1:
		for _, p := range sbs {
			log.Println(p)
		}
		return errors.New("found multiple superblocks")
	case len(sbs) < 1:
		return errors.New("no webfs superblocks found")
	default:
		log.Printf(`using "%s" as superblock`, sbs[0])
		superblockPath = sbs[0]
	}

	sb := webfs.SuperblockFromPath(superblockPath)
	wfs, err = webfs.New(sb, nil)
	if err != nil {
		return err
	}
	ctx = context.Background()
	return nil
}

func listSuperblocks() ([]string, error) {
	f, err := os.Open(".")
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	sbs := []string{}
	for _, name := range names {
		ext := filepath.Ext(name)
		if strings.ToLower(ext) == ".webfs" {
			sbs = append(sbs, name)
		}
	}
	return sbs, nil
}
