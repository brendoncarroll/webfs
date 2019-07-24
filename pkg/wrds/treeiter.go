package wrds

import (
	"bytes"
	"context"
)

func (t *Tree) Iterate(ctx context.Context, store Read, startKey []byte) (*TreeIter, error) {
	var lastKey []byte
	if len(startKey) > 0 {
		ent, err := t.MaxLteq(ctx, store, startKey)
		if err != nil {
			return nil, err
		}
		if ent != nil && bytes.Compare(ent.Key, startKey) != 0 {
			lastKey = ent.Key
		}
	}

	return &TreeIter{
		tree:    t,
		store:   store,
		lastKey: lastKey,
	}, nil
}

type TreeIter struct {
	store   Read
	tree    *Tree
	lastKey []byte
}

func (ti *TreeIter) Next(ctx context.Context) (*TreeEntry, error) {
	ent, err := ti.tree.MinGt(ctx, ti.store, ti.lastKey)
	if err != nil {
		return nil, err
	}
	if ent != nil {
		ti.lastKey = ent.Key
	}
	return ent, nil
}
