package wrds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Ref = webref.Ref
type WriteOnce = webref.WriteOnce
type Read = webref.Read
type ReadWriteOnce = webref.ReadWriteOnce

// a guess. a tree needs to be able to store at least 2 entries.
const minTreeSize = 1024

type TreeEntry struct {
	Key []byte `json:"key"`
	Ref Ref    `json:"ref"`
}

type Tree struct {
	Level   uint        `json:"level"`
	Entries []TreeEntry `json:"entries"`
}

func NewTree() *Tree {
	return &Tree{Level: 1}
}

func (t *Tree) Put(ctx context.Context, s ReadWriteOnce, key []byte, ref Ref) (*Tree, error) {
	ent := TreeEntry{Key: key, Ref: ref}
	newTree, err := t.put(ctx, s, ent)
	if err != nil {
		return nil, err
	}
	return newTree, nil
}

// Split forces the root to split.  The 2 subtrees are posted to s, and a new root is created and returned.
// Split should be called if the containing data structure can't fit a node into a blob.
func (t *Tree) Split(ctx context.Context, s ReadWriteOnce) (*Tree, error) {
	low, high := t.split()
	newTree := Tree{Level: t.Level + 1}

	for _, st := range []Tree{low, high} {
		data := st.Marshal()
		ref, err := s.Post(ctx, data)
		if err != nil {
			return nil, err
		}
		ent := TreeEntry{Key: st.MinKey(), Ref: *ref}
		newTree.Entries = append(newTree.Entries, ent)
	}
	return &newTree, nil
}

func (t *Tree) put(ctx context.Context, s ReadWriteOnce, ent TreeEntry) (*Tree, error) {
	i := t.indexOf(ent.Key)
	entries := []TreeEntry{}
	entries = append(entries, t.Entries[:i]...)

	if t.Level > 1 && i >= len(t.Entries) {
		return nil, errors.New("invalid tree: higher order tree with no entries")
	}

	// find subtree and recurse
	if t.Level > 1 {
		subTree := &Tree{}
		stData, err := s.Get(ctx, t.Entries[i].Ref)
		if err != nil {
			return nil, err
		}
		if err := subTree.Unmarshal(stData); err != nil {
			return nil, err
		}
		subTree, err = subTree.put(ctx, s, ent)
		if err != nil {
			return nil, err
		}

		subTrees := []Tree{*subTree}
		data := subTree.Marshal()
		// check if we need to split
		if len(data) > s.MaxBlobSize() {
			low, high := subTree.split()
			subTrees = []Tree{low, high}
		}
		// we either have one or 2 subtrees, post them all and convert to entries
		for _, st := range subTrees {
			data := st.Marshal()
			ref, err := s.Post(ctx, data)
			if err != nil {
				return nil, err
			}
			stEnt := TreeEntry{Key: st.MinKey(), Ref: *ref}
			entries = append(entries, stEnt)
		}
	} else {
		entries = append(entries, ent)
	}
	entries = append(entries, t.Entries[i:]...)
	newTree := Tree{Level: t.Level, Entries: entries}
	return &newTree, nil
}

func (t *Tree) indexOf(key []byte) int {
	i := sort.Search(len(t.Entries), func(i int) bool {
		return bytes.Compare(t.Entries[i].Key, key) >= 0
	})
	return i
}

// MaxLteq finds the max entry below key.
func (t *Tree) MaxLteq(ctx context.Context, s webref.Read, key []byte) (*TreeEntry, error) {
	i := t.indexOf(key)
	if i >= len(t.Entries) {
		return nil, nil
	}
	switch {
	case t.Level > 1:
		subtreeData, err := s.Get(ctx, t.Entries[i].Ref)
		if err != nil {
			return nil, err
		}
		subtree := Tree{}
		if err := subtree.Unmarshal(subtreeData); err != nil {
			return nil, err
		}
		return subtree.MaxLteq(ctx, s, key)
	default:
		entry := t.Entries[i]
		return &entry, nil
	}
}

func (t *Tree) MinGt(ctx context.Context, s Read, key []byte) (*TreeEntry, error) {
	i := sort.Search(len(t.Entries), func(i int) bool {
		return bytes.Compare(t.Entries[i].Key, key) > 0
	})
	switch {
	case i == len(t.Entries):
		return nil, nil
	case t.Level > 1:
		st, err := t.getSubtree(ctx, s, i)
		if err != nil {
			return nil, err
		}
		return st.MinGt(ctx, s, key)
	default:
		ent := t.Entries[i]
		return &ent, nil
	}

	return nil, nil
}

func (t *Tree) Get(ctx context.Context, s Read, key []byte) (*TreeEntry, error) {
	ent, err := t.MaxLteq(ctx, s, key)
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, nil
	}
	if bytes.Compare(ent.Key, key) == 0 {
		return ent, nil
	}
	return nil, nil
}

func merge(a, b Tree) Tree {
	t := Tree{}
	for _, e := range a.Entries {
		t.Entries = append(t.Entries, e)
	}
	for _, e := range b.Entries {
		t.Entries = append(t.Entries, e)
	}
	sort.SliceStable(t.Entries, func(i, j int) bool {
		return bytes.Compare(t.Entries[i].Key, t.Entries[j].Key) < 0
	})
	return t
}

func (t *Tree) split() (low, high Tree) {
	if len(t.Entries) < 2 {
		panic("split tree with < 2 entries")
	}
	low = Tree{}
	high = Tree{}

	mid := len(t.Entries) / 2
	lowEntries, highEntries := t.Entries[:mid], t.Entries[mid:]

	low.Entries = lowEntries
	high.Entries = highEntries
	return low, high
}

func (t *Tree) MinKey() []byte {
	if len(t.Entries) > 0 {
		return t.Entries[0].Key
	}
	return nil
}

func (t *Tree) MaxKey(ctx context.Context, s Read) ([]byte, error) {
	l := len(t.Entries)
	switch {
	case l < 1:
		return nil, nil
	case t.Level == 1:
		return t.Entries[l-1].Key, nil
	case t.Level > 1:
		subTree := Tree{}
		data, err := s.Get(ctx, t.Entries[l-1].Ref)
		if err != nil {
			return nil, err
		}
		if err := subTree.Unmarshal(data); err != nil {
			return nil, err
		}
		return subTree.MaxKey(ctx, s)
	default:
		return nil, errors.New("invalid tree: level < 1")
	}
}

func (t *Tree) Unmarshal(data []byte) error {
	return json.Unmarshal(data, t)
}

func (t *Tree) Marshal() []byte {
	data, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	return data
}

func (t *Tree) getSubtree(ctx context.Context, s Read, i int) (*Tree, error) {
	ent := t.Entries[i]
	data, err := s.Get(ctx, ent.Ref)
	if err != nil {
		return nil, err
	}
	subtree := Tree{}
	if err := subtree.Unmarshal(data); err != nil {
		return nil, err
	}
	return &subtree, nil
}

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