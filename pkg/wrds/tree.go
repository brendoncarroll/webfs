package wrds

import (
	"bytes"
	"context"
	"errors"
	fmt "fmt"
	"sort"

	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Ref = webref.Ref

type Read = webref.Getter
type Post = webref.Poster
type ReadPost interface {
	webref.Getter
	webref.Poster
}

const (
	// minimum entries to split
	splitMinEntries = 4
)

func NewTree() *Tree {
	return &Tree{Level: 1}
}

func (t *Tree) Put(ctx context.Context, s ReadPost, key []byte, ref *Ref) (*Tree, error) {
	ent := &TreeEntry{Key: key, Ref: ref}
	newTree, err := t.put(ctx, s, ent)
	if err != nil {
		return nil, err
	}
	return newTree, nil
}

// Split forces the root to split.  The 2 subtrees are posted to s, and a new root is created and returned.
// Split should be called if the containing data structure can't fit into a blob.
func (t *Tree) Split(ctx context.Context, s Post) (*Tree, error) {
	newTree := &Tree{Level: t.Level + 1}

	low, high := t.split()
	subTrees := []Tree{high, low}
	for len(subTrees) > 0 {
		st := subTrees[len(subTrees)-1]
		subTrees = subTrees[:len(subTrees)-1]

		ref, err := webref.EncodeAndPost(ctx, s, &st)
		if err == webref.ErrMaxSizeExceeded {
			if len(st.Entries) < splitMinEntries {
				return nil, fmt.Errorf("cannot further split tree")
			}
			l2, h2 := st.split()
			subTrees = append(subTrees, []Tree{h2, l2}...)
			continue
		}
		if err != nil {
			return nil, err
		}

		ent := &TreeEntry{Key: st.MinKey(), Ref: ref}
		newTree.Entries = append(newTree.Entries, ent)
	}

	if sort.SliceIsSorted(newTree.Entries, func(i, j int) bool {
		return bytes.Compare(newTree.Entries[i].Key, newTree.Entries[j].Key) < 0
	}) {
		panic("unsorted entries")
	}

	return newTree, nil
}

func (t *Tree) put(ctx context.Context, s ReadPost, ent *TreeEntry) (*Tree, error) {
	i := t.indexPut(ent.Key)

	entries := []*TreeEntry{}
	entries = append(entries, t.Entries[:i]...)

	if t.Level > 1 && i < len(t.Entries) {
		return nil, errors.New("invalid tree: higher order tree with no entries")
	}

	// find subtree and recurse
	if t.Level > 1 {
		subTree := &Tree{}
		err := webref.GetAndDecode(ctx, s, *t.Entries[i].Ref, subTree)
		if err != nil {
			return nil, err
		}

		subTree, err = subTree.put(ctx, s, ent)
		if err != nil {
			return nil, err
		}

		subTrees := []Tree{*subTree}
		// check if we need to split
		codec := webref.GetCodecCtx(ctx)
		if webref.SizeOf(codec, subTree) > s.MaxBlobSize() {
			low, high := subTree.split()
			subTrees = []Tree{low, high}
		}
		// we either have one or 2 subtrees, post them all and convert to entries
		for _, st := range subTrees {
			ref, err := webref.EncodeAndPost(ctx, s, st)
			if err != nil {
				return nil, err
			}
			stEnt := &TreeEntry{Key: st.MinKey(), Ref: ref}
			entries = append(entries, stEnt)
		}
	} else {
		entries = append(entries, ent)
	}

	entries = append(entries, t.Entries[i:]...)
	newTree := Tree{Level: t.Level, Entries: entries}
	return &newTree, nil
}

func (t *Tree) indexPut(key []byte) int {
	var i int
	for i = 0; i < len(t.Entries); i++ {
		e := t.Entries[i]
		cmp := bytes.Compare(e.Key, key)
		if cmp >= 0 {
			break
		}
	}
	return i
}

func (t *Tree) indexGet(key []byte) int {
	var (
		i = -1
		e *TreeEntry
	)
	for i, e = range t.Entries {
		cmp := bytes.Compare(e.Key, key)

		switch {
		case cmp > 0:
			return i - 1
		case cmp == 0:
			return i
		}
	}
	return i
}

// MaxLteq finds the max entry below key.
func (t *Tree) MaxLteq(ctx context.Context, s Read, key []byte) (*TreeEntry, error) {
	i := t.indexGet(key)
	if i == -1 {
		return nil, nil
	}
	switch {
	case t.Level > 1:
		subtree := &Tree{}
		err := webref.GetAndDecode(ctx, s, *t.Entries[i].Ref, subtree)
		if err != nil {
			return nil, err
		}
		return subtree.MaxLteq(ctx, s, key)
	case t.Level == 1:
		entry := t.Entries[i]
		return entry, nil
	default:
		return nil, errors.New("invalid tree")
	}
}

// MinGt (After)
func (t *Tree) MinGt(ctx context.Context, s Read, key []byte) (*TreeEntry, error) {
	i := 0
	// find the max index with at least 1 key
	// TODO: binary search
	for ; i < len(t.Entries); i++ {
		if bytes.Compare(t.Entries[i].Key, key) > 0 {
			break
		}
	}

	lt := i - 1
	gt := i
	switch {
	case t.Level == 1 && gt < len(t.Entries):
		return t.Entries[gt], nil
	case t.Level == 1 && gt >= len(t.Entries):
		return nil, nil
	case t.Level > 1:
		var ret *TreeEntry
		if lt >= 0 {
			st, err := t.getSubtree(ctx, s, lt)
			if err != nil {
				return nil, err
			}
			ent, err := st.MinGt(ctx, s, key)
			if err != nil {
				return nil, err
			}
			ret = ent
		}
		if ret == nil && gt < len(t.Entries) {
			st, err := t.getSubtree(ctx, s, gt)
			if err != nil {
				return nil, err
			}
			ent, err := st.MinGt(ctx, s, key)
			if err != nil {
				return nil, err
			}
			ret = ent
		}
		return ret, nil
	default:
		return nil, errors.New("invalid tree")
	}
}

func (t *Tree) Get(ctx context.Context, s Read, key []byte) (*TreeEntry, error) {
	i := t.indexGet(key)
	if i < 0 {
		return nil, nil
	}

	switch {
	case t.Level > 1:
		subTree, err := t.getSubtree(ctx, s, i)
		if err != nil {
			return nil, err
		}
		return subTree.Get(ctx, s, key)
	case t.Level == 1:
		ent := t.Entries[i]
		if bytes.Compare(key, ent.Key) == 0 {
			return ent, nil
		}
		return nil, nil
	default:
		return nil, errors.New("Invalid tree level")
	}
}

func (t *Tree) Delete(ctx context.Context, s ReadPost, key []byte) (*Tree, error) {
	return t.delete(ctx, s, key)
}

func (t *Tree) delete(ctx context.Context, s ReadPost, key []byte) (*Tree, error) {
	// TODO: not balanced
	i := t.indexGet(key)
	if i < 0 {
		return t, nil
	}

	newEntries := []*TreeEntry{}
	switch {
	case t.Level > 1:
		subTree, err := t.getSubtree(ctx, s, i)
		if err != nil {
			return nil, err
		}

		newSt, err := subTree.delete(ctx, s, key)
		if err != nil {
			return nil, err
		}
		ref, err := webref.EncodeAndPost(ctx, s, newSt)
		if err != nil {
			return nil, err
		}

		ent := &TreeEntry{Key: subTree.MinKey(), Ref: ref}
		newEntries = append(newEntries, t.Entries[:i]...)
		newEntries = append(newEntries, ent)
		if i < len(t.Entries)-1 {
			newEntries = append(newEntries, t.Entries[i+1:]...)
		}

	case t.Level == 1:
		ent := t.Entries[i]
		if bytes.Compare(key, ent.Key) == 0 {
			newEntries = append(newEntries, t.Entries[:i]...)
			if i < len(t.Entries)-1 {
				newEntries = append(newEntries, t.Entries[i+1:]...)
			}
		} else {
			return t, nil
		}
	default:
		return nil, errors.New("Invalid tree level")
	}

	newTree := &Tree{
		Level:   t.Level,
		Entries: newEntries,
	}
	return newTree, nil

	switch {
	case t.Level == 1:
		newEntries := []*TreeEntry{}
		newEntries = append(newEntries, t.Entries[:i]...)
		newEntries = append(newEntries, t.Entries[i+1:]...)
		if len(newEntries) == 0 {
			return nil, nil
		}
		newTree := Tree{Entries: newEntries}
		return &newTree, nil
	case t.Level > 1:
		subTree, err := t.getSubtree(ctx, s, i)
		if err != nil {
			return nil, err
		}
		newSt, err := subTree.delete(ctx, s, key)
		if err != nil {
			return nil, err
		}

		newEntries := []*TreeEntry{}
		newEntries = append(newEntries, t.Entries[:i]...)
		if newSt != nil {
			ref, err := webref.EncodeAndPost(ctx, s, newSt)
			if err != nil {
				return nil, err
			}
			newEnt := &TreeEntry{Key: t.Entries[i].Key, Ref: ref}
			newEntries = append(newEntries, newEnt)
		}
		newEntries = append(newEntries, t.Entries[i+1:]...)
		newTree := Tree{Level: t.Level, Entries: newEntries}
		return &newTree, nil
	default:
		return nil, errors.New("invalid tree level")
	}
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
		subTree := &Tree{}
		err := webref.GetAndDecode(ctx, s, *t.Entries[l-1].Ref, subTree)
		if err != nil {
			return nil, err
		}
		return subTree.MaxKey(ctx, s)
	default:
		return nil, errors.New("invalid tree: level < 1")
	}
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

func (t *Tree) getSubtree(ctx context.Context, s Read, i int) (*Tree, error) {
	ent := t.Entries[i]
	subtree := &Tree{}

	err := webref.GetAndDecode(ctx, s, *ent.Ref, subtree)
	if err != nil {
		return nil, err
	}
	return subtree, nil
}
