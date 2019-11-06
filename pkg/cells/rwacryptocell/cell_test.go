package rwacryptocell

import (
	"context"
	fmt "fmt"
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
			AuxState:      memcell.New(),
		}
		c := New(spec)
		err = c.Claim(ctx)
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
		AuxState:      memcell.New(),
	}

	c := New(spec)
	err = c.Claim(ctx)
	require.Nil(t, err)

	data, err := c.Get(ctx)
	require.Nil(t, err)
	assert.Len(t, data, 0)

	success, err := c.CAS(ctx, nil, []byte("test data"))
	require.Nil(t, err)
	assert.True(t, success)
	t.Log(c.Inspect(ctx))

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
		AuxState:      memcell.New(),
	}

	c := New(spec)
	err = c.Claim(ctx)
	require.Nil(t, err)

	peerPriv, err := GenerateEntity()
	require.Nil(t, err)
	peerPub := GetPublicEntity(peerPriv)

	err = c.AddEntity(ctx, peerPub)
	require.Nil(t, err)
	err = c.GrantRead(ctx, peerPub)
	require.Nil(t, err)
}

func TestShare(t *testing.T) {
	c := memcell.New()
	sides := make([]*Side, 3)
	for i := range sides {
		sides[i] = newSide(c)
	}

	ctx := context.TODO()
	err := sides[0].C.Claim(ctx)
	require.Nil(t, err)

	adminS := sides[0]
	for i := 1; i < len(sides); i++ {
		s := sides[i]

		err = adminS.C.AddEntity(ctx, s.Public)
		require.Nil(t, err)
		err = adminS.C.GrantWrite(ctx, s.Public)
		require.Nil(t, err)
		err = adminS.C.GrantRead(ctx, s.Public)
		require.Nil(t, err)

		err = s.C.Join(ctx, func(interface{}) bool {
			return true
		})
		require.Nil(t, err)
		assert.NotEmpty(t, s.AuxState.GetOrDie())
	}

	t.Log(adminS.C.Inspect(ctx))
	for i, s := range sides {
		cur, err := s.C.Get(ctx)
		require.Nil(t, err)
		next := []byte(fmt.Sprint("side", i))
		success, err := s.C.CAS(ctx, cur, next)
		require.Nil(t, err)
		assert.True(t, success)
	}

	for _, s := range sides {
		cur, err := s.C.Get(ctx)
		require.Nil(t, err)
		assert.Equal(t, fmt.Sprint("side", len(sides)-1), string(cur))
	}
}

func newSide(inner cells.Cell) *Side {
	priv, err := GenerateEntity()
	if err != nil {
		panic(err)
	}
	pub := GetPublicEntity(priv)

	auxState := memcell.New()
	spec := Spec{
		Inner:         inner,
		PublicEntity:  pub,
		PrivateEntity: priv,
		AuxState:      auxState,
	}
	cell := New(spec)
	return &Side{
		Private:  priv,
		Public:   pub,
		AuxState: auxState,
		C:        cell,
	}
}

type Side struct {
	Private, Public *Entity
	C               *Cell
	AuxState        *memcell.Cell
}
