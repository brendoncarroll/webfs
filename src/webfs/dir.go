package webfs

import (
	"context"
	"fmt"
	"io/fs"
	"iter"
)

type DirEnt struct {
	Name   string
	Mode   fs.FileMode
	Target INode
	Owner inet256.ID
}

func (tx *Tx) InspectDir(ctx context.Context, ino INode) (Dir, error) {
	node, err := tx.GetNode(ctx, ino)
	if err != nil {
		return Dir{}, err
	}
	if node.Type != TC_DIR {
		return Dir{}, fmt.Errorf("node %v is not a directory", ino)
	}
	return Dir{}, nil
}

// CreateDirAt creates a new empty directory, and adds it to the parent.
func (tx *Tx) CreateDirAt(ctx context.Context, parent INode, name string) (INode, error) {
	dir := Dir{}
}

func (tx *Tx) Link(ctx context.Context, ino INode, name string, mode fs.FileMode, child INode) error {
	dinfo,
	return nil
}

func (tx *Tx) ReadDir(ctx context.Context, ino INode, gteq string) iter.Seq2[DirEnt, error] {
	return func(yield func(DirEnt, error) bool) {}
}

func makeDirEntKey(out []byte, ino INode, name string) []byte {
	out = append(out, ino[:]...)
	out = append(out, name...)
	return out
}
