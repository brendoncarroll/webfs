package wrds

import (
	"context"
	"fmt"
	"testing"

	"github.com/brendoncarroll/webfs/pkg/webref"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = context.TODO()

func TestIndexOf(t *testing.T) {
	tree := Tree{
		Entries: []*TreeEntry{
			{Key: []byte("bbb")},
			{Key: []byte("ccc")},
			{Key: []byte("ddd")},
		},
	}
	var i int
	i = tree.indexPut([]byte("aaa"))
	assert.Equal(t, 0, i)
	i = tree.indexPut([]byte("eee"))
	assert.Equal(t, 3, i)
}

func TestPutGet(t *testing.T) {
	const N = 10
	s := webref.NewMemStore()

	tree := NewTree()
	var err error
	for i := 0; i < N; i++ {
		key := []byte(fmt.Sprintf("key%03d", i))
		ref := &webref.Ref{}
		tree, err = tree.Put(ctx, s, key, ref)
		require.Nil(t, err)
	}
	t.Log("tree root has", len(tree.Entries), "entries")
	t.Log("tree entries", tree.Entries)

	for i := 0; i < N; i++ {
		key := []byte(fmt.Sprintf("key%03d", i))
		ent, err := tree.Get(ctx, s, key)
		require.Nil(t, err)
		if assert.NotNil(t, ent) {
			assert.Equal(t, key, ent.Key)
		} else {
			t.Log("missing", string(key))
		}
	}
}
