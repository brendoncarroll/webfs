package webfs

import (
	"context"
	"testing"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/webref"
	"github.com/brendoncarroll/webfs/pkg/wrds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileAppend(t *testing.T) {
	ctx := context.TODO()
	opts := webref.DefaultOptions()
	store := stores.NewMemStore()
	f := &models.File{Tree: wrds.NewTree()}

	testData := []string{
		"1. hello\n",
		"2. hello again\n",
		"this is the third line\n",
	}

	var (
		n   int
		err error
	)
	for _, s := range testData {
		d := []byte(s)
		f, err = fileAppend(ctx, store, *opts, *f, d)
		n += len(d)
		require.Nil(t, err)
	}
	assert.Len(t, f.Tree.Entries, 3)
	assert.Equal(t, f.Size, uint64(n))
}
