package webfsim

import (
	"context"
	"errors"
	"strings"

	webref "github.com/brendoncarroll/webfs/pkg/webref"
)

type ReadPost interface {
	webref.Getter
	webref.Poster
}

type Getter = webref.Getter

func DirGet(ctx context.Context, store Getter, m Dir, name string) (*DirEntry, error) {
	key := []byte(name)
	treeEnt, err := m.Tree.Get(ctx, store, key)
	if err != nil {
		return nil, err
	}
	if treeEnt == nil {
		return nil, nil
	}

	dirEnt := DirEntry{}
	if err := webref.GetAndDecode(ctx, store, *treeEnt.Ref, &dirEnt); err != nil {
		return nil, err
	}
	return &dirEnt, nil
}

func DirPut(ctx context.Context, store ReadPost, m Dir, ent DirEntry) (*Dir, error) {
	if strings.Contains(ent.Name, "/") {
		return nil, errors.New("directory name contains a slash")
	}

	var (
		ref *webref.Ref
		err error
	)
	for {
		ref, err = webref.EncodeAndPost(ctx, store, &ent)
		if err == webref.ErrMaxSizeExceeded {
			ent, err = DirEntSplit(ctx, store, ent)
			if err != nil {
				return nil, err
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}

	key := []byte(ent.Name)
	tree, err := m.Tree.Put(ctx, store, key, ref)
	if err != nil {
		return nil, err
	}
	return &Dir{Tree: tree}, nil
}

func DirDelete(ctx context.Context, store ReadPost, m Dir, name string) (*Dir, error) {
	key := []byte(name)
	newTree, err := m.Tree.Delete(ctx, store, key)
	if err != nil {
		return nil, err
	}
	return &Dir{Tree: newTree}, nil
}

func DirSplit(ctx context.Context, store ReadPost, x Dir) (*Dir, error) {
	newTree, err := x.Tree.Split(ctx, store)
	if err != nil {
		return nil, err
	}
	return &Dir{Tree: newTree}, nil
}

func DirEntSplit(ctx context.Context, store ReadPost, x DirEntry) (DirEntry, error) {
	o2, err := ObjectSplit(ctx, store, *x.Object)
	if err != nil {
		return DirEntry{}, nil
	}
	return DirEntry{Object: o2, Name: x.Name}, nil
}
