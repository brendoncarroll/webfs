package wrds

import (
	"bytes"
	"context"

	"github.com/brendoncarroll/webfs/pkg/webref"
)

type TreeBuilder struct {
	trees   []*Tree
	lastKey []byte
	isDone  bool
}

func NewTreeBuilder() *TreeBuilder {
	return &TreeBuilder{
		trees: []*Tree{NewTree()},
	}
}

func (tb *TreeBuilder) Put(ctx context.Context, s ReadWriteOnce, opts webref.Options, ent *TreeEntry) error {
	if bytes.Compare(ent.Key, tb.lastKey) <= 0 {
		panic("tree builder: Put called with out of order key")
	}
	if tb.isDone {
		panic("tree builder: Put called after Finish")
	}
	nextLastKey := ent.Key

	for i := 0; i < len(tb.trees); i++ {
		t := tb.trees[i]

		// try to insert to current tree
		t.Entries = append(t.Entries, ent)
		if webref.SizeOf(s, opts, t) < s.MaxBlobSize() {
			break
		}

		// if its too big then undo
		t.Entries = t.Entries[:len(t.Entries)-1]
		newTree := &Tree{
			Level:   uint32(i + 1),
			Entries: []*TreeEntry{ent},
		}
		tb.trees[0] = newTree

		// store the last tree
		key := t.MinKey()
		ref, err := webref.Store(ctx, s, opts, t)
		if err != nil {
			return err
		}

		// create the next level tree if it's not already there
		if len(tb.trees) < i+2 {
			tb.trees = append(tb.trees, &Tree{Level: uint32(i + 2)})
		}

		// change the entry to refer to tree that was just posted
		ent = &TreeEntry{Key: key, Ref: ref}
	}

	tb.lastKey = nextLastKey
	return nil
}

func (tb *TreeBuilder) Finish(ctx context.Context, store ReadWriteOnce, opts Options) (*Tree, error) {
	if tb.isDone {
		panic("Finish called twice")
	}
	tb.isDone = true

	var ent *TreeEntry
	for _, t := range tb.trees {
		if ent != nil {
			t.Entries = append(t.Entries, ent)
		}
		ref, err := webref.Store(ctx, store, opts, t)
		if err != nil {
			return nil, err
		}
		ent = &TreeEntry{
			Key: t.MinKey(),
			Ref: ref,
		}
	}

	l := len(tb.trees)
	return tb.trees[l-1], nil
}
