package cryptocell

import (
	"context"
	"testing"

	"github.com/brendoncarroll/webfs/pkg/cells/memcell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCAS(t *testing.T) {
	ctx := context.TODO()
	mc := memcell.New()
	privEnt, err := GenerateEntity()
	require.Nil(t, err)
	pubEnt := GetPublicEntity(privEnt)

	spec := Spec{
		Inner: mc,
		Who: &Who{
			Entities: []*Entity{pubEnt},
			Admin:    []int32{0},
			Write:    []int32{0},
			Read:     []int32{0},
		},
		PrivateEntity: privEnt,
		PublicEntity:  pubEnt,
	}

	c := New(spec)
	err = c.init(ctx, 0)
	require.Nil(t, err)

	data, err := c.Get(ctx)
	require.Nil(t, err)
	assert.Len(t, data, 0)

	success, err := c.CAS(ctx, nil, []byte("test data"))
	require.Nil(t, err)
	assert.True(t, success)
	t.Log(mc.GetOrDie())

	data, err = c.Get(ctx)
	require.Nil(t, err)
	assert.Equal(t, []byte("test data"), data)
}
