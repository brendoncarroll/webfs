package webfsim

import (
	"context"
	"errors"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/stores"
	webref "github.com/brendoncarroll/webfs/pkg/webref"
)

func DirGet(ctx context.Context, store stores.Read, m Dir, name string) (*DirEntry, error) {
	key := []byte(name)
	treeEnt, err := m.Tree.Get(ctx, store, key)
	if err != nil {
		return nil, err
	}
	if treeEnt == nil {
		return nil, nil
	}

	dirEnt := DirEntry{}
	if err := webref.Load(ctx, store, *treeEnt.Ref, &dirEnt); err != nil {
		return nil, err
	}
	return &dirEnt, nil
}

func DirPut(ctx context.Context, store stores.ReadPost, opts webref.Options, m Dir, ent DirEntry) (*Dir, error) {
	if strings.Contains(ent.Name, "/") {
		return nil, errors.New("directory name contains a slash")
	}

	var (
		ref *webref.Ref
		err error
	)
	for {
		ref, err = webref.Store(ctx, store, opts, &ent)
		if err == webref.ErrMaxSizeExceeded {
			ent, err = DirEntSplit(ctx, store, opts, ent)
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
	tree, err := m.Tree.Put(ctx, store, opts, key, ref)
	if err != nil {
		return nil, err
	}
	return &Dir{Tree: tree}, nil
}

func DirDelete(ctx context.Context, store stores.ReadPost, opts webref.Options, m Dir, name string) (*Dir, error) {
	key := []byte(name)
	newTree, err := m.Tree.Delete(ctx, store, opts, key)
	if err != nil {
		return nil, err
	}
	return &Dir{Tree: newTree}, nil
}

func DirSplit(ctx context.Context, store stores.ReadPost, opts webref.Options, x Dir) (*Dir, error) {
	newTree, err := x.Tree.Split(ctx, store, opts)
	if err != nil {
		return nil, err
	}
	return &Dir{Tree: newTree}, nil
}

func DirEntSplit(ctx context.Context, store stores.ReadPost, opts webref.Options, x DirEntry) (DirEntry, error) {
	o2, err := ObjectSplit(ctx, store, opts, *x.Object)
	if err != nil {
		return DirEntry{}, nil
	}
	return DirEntry{Object: o2, Name: x.Name}, nil
}
