package merkleds

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"

	"github.com/brendoncarroll/webfs/pkg/l2"
)

type Ref = l2.Ref
type WriteOnce = l2.WriteOnce
type Read = l2.Read
type ReadWriteOnce = l2.ReadWriteOnce

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
	// this is wrong, but it seems to work
	i := t.indexOf(key)
	entries := []TreeEntry{}

	if t.Level > 1 {
		subTree := &Tree{}
		stData, err := s.Get(ctx, t.Entries[i].Ref)
		if err != nil {
			return nil, err
		}
		if err := subTree.Unmarshal(stData); err != nil {
			return nil, err
		}
		subTree, err = subTree.Put(ctx, s, key, ref)
		if err != nil {
			return nil, err
		}
		stData = subTree.Marshal()
		stRef, err := s.Post(ctx, stData)
		if err != nil {
			return nil, err
		}
		ref = *stRef
	}

	entries = []TreeEntry{
		{Key: key, Ref: ref},
	}

	// insert into this node
	ret := Tree{Level: t.Level}
	ret.Entries = append(ret.Entries, t.Entries[:i]...)
	ret.Entries = append(ret.Entries, entries...)
	if (i + 1) < len(t.Entries) {
		ret.Entries = append(ret.Entries, t.Entries[i+1:]...)
	}

	// see if we need to split
	data := ret.Marshal()
	maxSize := s.MaxBlobSize()
	if maxSize < minTreeSize {
		panic("maxSize too small")
	}
	if len(data) > maxSize {
		low, high := ret.split()
		subtrees := []*Tree{low, high}
		entries := []TreeEntry{}
		for _, subtree := range subtrees {
			data := subtree.Marshal()
			stRef, err := s.Post(ctx, data)
			if err != nil {
				return nil, err
			}
			entry := TreeEntry{
				Key: subtree.MinKey(),
				Ref: *stRef,
			}
			entries = append(entries, entry)
		}
		ret = Tree{Level: ret.Level + 1, Entries: entries}
	}

	return &ret, nil
}

func (t *Tree) indexOf(key []byte) int {
	i := sort.Search(len(t.Entries), func(i int) bool {
		return bytes.Compare(t.Entries[i].Key, key) >= 0
	})
	return i
}

// MaxLteq finds the max entry below key.
func (t *Tree) MaxLteq(ctx context.Context, s l2.Read, key []byte) (*TreeEntry, error) {
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

func merge(ctx context.Context, s WriteOnce, a, b *Tree) (*Tree, error) {
	t := &Tree{}
	for _, e := range a.Entries {
		t.Entries = append(t.Entries, e)
	}
	for _, e := range b.Entries {
		t.Entries = append(t.Entries, e)
	}
	return t, nil
}

func (t *Tree) split() (low, high *Tree) {
	low = &Tree{}
	high = &Tree{}

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

func (t *Tree) Iterate(store Read) *TreeIter {
	return &TreeIter{
		tree:  t,
		store: store,
	}
	return nil
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
