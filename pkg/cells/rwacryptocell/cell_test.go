package rwacryptocell

import (
	"context"
	"testing"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/memcell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCell(t *testing.T) {
	cells.CellTestSuite(t, func() cells.Cell {
		ctx := context.TODO()
		privEnt, err := GenerateEntity()
		if err != nil {
			panic(err)
		}
		pubEnt := GetPublicEntity(privEnt)
		spec := Spec{
			Inner:         memcell.New(),
			PrivateEntity: privEnt,
			PublicEntity:  pubEnt,
		}
		auxState := memcell.New()
		c := New(spec, auxState)
		err = c.Init(ctx)
		if err != nil {
			panic(err)
		}
		return c
	})
}

func TestCAS(t *testing.T) {
	ctx := context.TODO()
	privEnt, err := GenerateEntity()
	require.Nil(t, err)
	pubEnt := GetPublicEntity(privEnt)
	mc := memcell.New()
	spec := Spec{
		Inner:         mc,
		PrivateEntity: privEnt,
		PublicEntity:  pubEnt,
	}

	auxState := memcell.New()
	c := New(spec, auxState)
	err = c.Init(ctx)
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

func TestAddPeer(t *testing.T) {
	ctx := context.TODO()
	privEnt, err := GenerateEntity()
	require.Nil(t, err)
	pubEnt := GetPublicEntity(privEnt)
	mc := memcell.New()
	spec := Spec{
		Inner:         mc,
		PrivateEntity: privEnt,
		PublicEntity:  pubEnt,
	}

	auxState := memcell.New()
	c := New(spec, auxState)
	err = c.Init(ctx)
	require.Nil(t, err)

	peerPriv, err := GenerateEntity()
	require.Nil(t, err)
	peerPub := GetPublicEntity(peerPriv)

	err = c.AddEntity(ctx, peerPub)
	require.Nil(t, err)
	err = c.GrantRead(ctx, peerPub)
	require.Nil(t, err)
}

type Side struct {
	Private, Public *Entity
	C               *Cell
	AuxState        *memcell.Cell
}
