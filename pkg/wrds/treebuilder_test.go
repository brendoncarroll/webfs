package wrds

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webref"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestTreeBuilder(t *testing.T) {
	ctx := context.TODO()
	tb := NewTreeBuilder()
	store := &webref.BasicStore{stores.NewMemStore(4096)}

	dataRef, err := store.Post(ctx, make([]byte, 1024))
	require.Nil(t, err)

	const N = 1000
	for i := 0; i < N; i++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(i))

		err := tb.Put(ctx, store, &TreeEntry{Key: key, Ref: dataRef})
		require.Nil(t, err)
	}

	t.Log(tb)
	tree, err := tb.Finish(ctx, store)
	require.Nil(t, err)
	require.NotNil(t, tree)
	t.Log(tree)
	for i := 0; i < N; i++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(i))
		te, err := tree.Get(ctx, store, key)
		require.Nil(t, err)
		require.NotNil(t, te)
		require.True(t, proto.Equal(te.Ref, dataRef))
	}
}
