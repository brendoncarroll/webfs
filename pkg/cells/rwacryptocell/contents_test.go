package rwacryptocell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	privEnt, err := GenerateEntity()
	require.Nil(t, err)

	cc := &CellContents{}
	cc, err = AddEntity(cc, privEnt, GetPublicEntity(privEnt))
	require.Nil(t, err)
	cc, err = AddAdmin(cc, privEnt, GetPublicEntity(privEnt))
	require.Nil(t, err)

	config := Spec{
		Who: &Who{
			Entities: []*Entity{GetPublicEntity(privEnt)},
			Admin:    []int32{0},
			Write:    nil,
			Read:     nil,
		},
		PrivateEntity: privEnt,
	}
	errs := ValidateContents(config, cc)
	assert.Len(t, errs, 0)

	rando, err := GenerateEntity()
	require.Nil(t, err)
	cc.Who.Entities = append(cc.Who.Entities, GetPublicEntity(rando))

	errs = ValidateContents(config, cc)
	assert.NotEmpty(t, errs)
}
