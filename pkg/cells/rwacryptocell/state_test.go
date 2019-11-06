package rwacryptocell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	privEnt, err := GenerateEntity()
	require.Nil(t, err)

	cc := &CellState{}
	cc, err = AddEntity(cc, privEnt, GetPublicEntity(privEnt))
	require.Nil(t, err)
	cc, err = AddAdmin(cc, privEnt, GetPublicEntity(privEnt))
	require.Nil(t, err)

	initACL := &ACL{
		Entities: []*Entity{GetPublicEntity(privEnt)},
		Admin:    []int32{0},
		Write:    nil,
		Read:     nil,
	}
	errs := ValidateState(initACL, cc)
	assert.Len(t, errs, 0)

	rando, err := GenerateEntity()
	require.Nil(t, err)
	cc.Acl.Entities = append(cc.Acl.Entities, GetPublicEntity(rando))

	errs = ValidateState(initACL, cc)
	assert.NotEmpty(t, errs)
}
