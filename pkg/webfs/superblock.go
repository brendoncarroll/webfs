package webfs

import (
	"os"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/filecell"
)

func NewSuperblock(p string) (Cell, error) {
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return SuperblockFromPath(p), nil
}

func SuperblockFromPath(p string) Cell {
	c := cells.Make(filecell.Spec{Path: p})
	return c
}
