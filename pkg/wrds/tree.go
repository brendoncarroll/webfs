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
	key2 := make([]byte, len(key))
	copy(key2, key)
	ent := &TreeEntry{Key: key2, Ref: ref}

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
	switch {
	case t.Level > 1 && len(t.Entries) < 1:
		return nil, errors.New("invalid tree: higher order tree with no entries")
	case t.Level > 1:
		return t.replaceSubtree(ctx, s, ent.Key, func(x *Tree) (*Tree, error) {
			return x.put(ctx, s, ent)
		})
	case t.Level == 1:
		t2 := &Tree{
			Level:   t.Level,
			Entries: slicePut(t.Entries, ent),
		}
		return t2, nil
	default:
		return nil, errors.New("invalid tree level")
	}
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
			st, err := t.getSubtreeAt(ctx, s, lt)
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
			st, err := t.getSubtreeAt(ctx, s, gt)
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
		return nil, errors.New("invalid tree level")
	}
}

func (t *Tree) Get(ctx context.Context, s Read, key []byte) (*TreeEntry, error) {
	switch {
	case t.Level > 1:
		subTree, err := t.getSubtree(ctx, s, key)
		if err != nil {
			return nil, err
		}
		if subTree == nil {
			return nil, err
		}
		return subTree.Get(ctx, s, key)
	case t.Level == 1:
		i := t.indexGet(key)
		if i < 0 {
			return nil, nil
		}
		ent := t.Entries[i]
		if bytes.Compare(key, ent.Key) == 0 {
			return ent, nil
		}
		return nil, nil
	default:
		return nil, errors.New("invalid tree level")
	}
}

func (t *Tree) Delete(ctx context.Context, s ReadPost, key []byte) (*Tree, error) {
	return t.delete(ctx, s, key)
}

func (t *Tree) delete(ctx context.Context, s ReadPost, key []byte) (*Tree, error) {
	// TODO: not balanced
	switch {
	case t.Level > 1:
		return t.replaceSubtree(ctx, s, key, func(x *Tree) (*Tree, error) {
			return x.delete(ctx, s, key)
		})
	case t.Level == 1:
		t2 := &Tree{
			Level:   t.Level,
			Entries: sliceDelete(t.Entries, key),
		}
		return t2, nil
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

func (t *Tree) replaceSubtree(ctx context.Context, s ReadPost, key []byte, fn func(t *Tree) (*Tree, error)) (*Tree, error) {
	i := t.indexPut(key)
	ent := t.Entries[i]

	st := &Tree{}
	if err := webref.GetAndDecode(ctx, s, *ent.Ref, st); err != nil {
		return nil, err
	}

	st2, err := fn(t)
	if err != nil {
		return nil, err
	}

	// check if we need to split
	codec := webref.GetCodecCtx(ctx)
	subTrees := []Tree{*st2}
	if webref.SizeOf(codec, st2) > s.MaxBlobSize() {
		low, high := st2.split()
		subTrees = []Tree{low, high}
	}
	// we either have one or 2 subtrees, post them all and convert to entries
	entries := t.Entries
	for _, st := range subTrees {
		ref, err := webref.EncodeAndPost(ctx, s, st)
		if err != nil {
			return nil, err
		}
		stEnt := &TreeEntry{Key: st.MinKey(), Ref: ref}
		entries = slicePut(entries, stEnt)
	}

	t2 := &Tree{
		Level:   t.Level,
		Entries: entries,
	}
	return t2, nil
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

func (t *Tree) getSubtree(ctx context.Context, s Read, key []byte) (*Tree, error) {
	i := t.indexGet(key)
	return t.getSubtreeAt(ctx, s, i)
}

func (t *Tree) getSubtreeAt(ctx context.Context, s Read, i int) (*Tree, error) {
	if i < 0 || i >= len(t.Entries) {
		return nil, nil
	}
	ent := t.Entries[i]
	subtree := &Tree{}

	err := webref.GetAndDecode(ctx, s, *ent.Ref, subtree)
	if err != nil {
		return nil, err
	}
	return subtree, nil
}

func slicePut(ents []*TreeEntry, ent2 *TreeEntry) []*TreeEntry {
	ents2 := []*TreeEntry{}

	i := 0
	for _, ent := range ents {
		if bytes.Compare(ent.Key, ent2.Key) < 0 {
			ents2 = append(ents2, ent)
			i++
		} else {
			break
		}
	}
	ents2 = append(ents2, ent2)
	for _, ent := range ents[i:] {
		if bytes.Compare(ent.Key, ent2.Key) > 0 {
			ents2 = append(ents2, ent)
		}
	}

	return ents2
}

func sliceDelete(ents []*TreeEntry, key []byte) []*TreeEntry {
	ents2 := []*TreeEntry{}
	for _, ent := range ents {
		if bytes.Compare(ent.Key, key) != 0 {
			ents2 = append(ents2, ent)
		}
	}
	return ents2
}
