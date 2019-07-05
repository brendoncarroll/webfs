package webfs

import (
	"context"
	"testing"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/memcell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = context.TODO()

func TestNew(t *testing.T) {
	mc := cells.Make(memcell.Spec{})
	wfs, err := New(mc)
	require.Nil(t, err)

	o, err := wfs.Lookup(ctx, "")
	require.Nil(t, err)
	if assert.NotNil(t, o) {
		assert.Equal(t, o.Cell, mc)
		assert.Nil(t, o.Object.File)
		assert.Nil(t, o.Object.Cell)
		assert.NotNil(t, o.Object.Dir)
	}
}
