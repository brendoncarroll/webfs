package webfs

import (
	"context"
	"testing"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/memcell"
	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/stretchr/testify/require"
)

var ctx = context.TODO()

func TestNew(t *testing.T) {
	ms := stores.NewMemStore()
	mc := cells.Make(memcell.Spec{})
	wfs, err := New(mc, ms)
	require.Nil(t, err)

	o, err := wfs.Lookup(ctx, "")
	require.Nil(t, err)
	require.NotNil(t, o)
}
