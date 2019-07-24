package webfs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/brendoncarroll/webfs/pkg/webref"
	"github.com/brendoncarroll/webfs/pkg/wrds"
)

type Snapshot struct {
	Cell      CellSpec     `json:"cell"`
	Contents  CellContents `json:"contents"`
	Timestamp time.Time    `json:"timestamp"`
}

type Dir struct {
	Tree *wrds.Tree `json:"tree"`
}

type DirEntry struct {
	Name   string `json:"name"`
	Object Object `json:"object"`
}

func NewDir() *Dir {
	return &Dir{Tree: wrds.NewTree()}
}

func (d *Dir) Split(ctx context.Context, s ReadWriteOnce) (*Dir, error) {
	newTree, err := d.Tree.Split(ctx, s)
	if err != nil {
		return nil, err
	}
	return &Dir{Tree: newTree}, nil
}

func (d *Dir) Get(ctx context.Context, store Read, p string) (*Object, error) {
	parts := strings.SplitN(p, "/", 2)
	key := []byte(parts[0])
	treeEnt, err := d.Tree.Get(ctx, store, key)
	if err != nil {
		return nil, err
	}
	if treeEnt == nil {
		return nil, nil
	}
	data, err := store.Get(ctx, treeEnt.Ref)
	if err != nil {
		return nil, err
	}

	dirEnt := DirEntry{}
	if err := json.Unmarshal(data, &dirEnt); err != nil {
		return nil, err
	}

	return &dirEnt.Object, nil
}

func (d *Dir) Put(ctx context.Context, store ReadWriteOnce, ent DirEntry) (*Dir, error) {
	if strings.Contains(ent.Name, "/") {
		return nil, errors.New("directory name contains a slash")
	}

	data, _ := json.Marshal(ent)
	entRef, err := store.Post(ctx, data)
	if err != nil {
		return nil, err
	}
	key := []byte(ent.Name)
	tree, err := d.Tree.Put(ctx, store, key, *entRef)
	if err != nil {
		return nil, err
	}
	return &Dir{Tree: tree}, nil
}

func (d *Dir) Delete(ctx context.Context, store webref.ReadWriteOnce, name string) (*Dir, error) {
	key := []byte(name)
	newTree, err := d.Tree.Delete(ctx, store, key)
	if err != nil {
		return nil, err
	}
	return &Dir{Tree: newTree}, nil
}

func (d *Dir) Entries(ctx context.Context, store webref.Read) ([]DirEntry, error) {
	entries := []DirEntry{}
	iter, err := d.Tree.Iterate(ctx, store, nil)
	if err != nil {
		return nil, err
	}

	for {
		ent, err := iter.Next(ctx)
		if err != nil {
			return nil, err
		}
		if ent == nil {
			break
		}
		dirEnt, err := getDirEnt(ctx, store, ent.Ref)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *dirEnt)
	}

	return entries, nil
}

func (d *Dir) RefIter(ctx context.Context, store Read, f func(Ref) bool) (bool, error) {
	return refIterTree(ctx, store, d.Tree, f)
}

func getDirEnt(ctx context.Context, store Read, ref Ref) (*DirEntry, error) {
	data, err := store.Get(ctx, ref)
	if err != nil {
		return nil, err
	}

	dirEnt := DirEntry{}
	if err := json.Unmarshal(data, &dirEnt); err != nil {
		return nil, err
	}
	return &dirEnt, nil
}
