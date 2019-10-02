package webfs

import (
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/httpcell"
	rwacryptocell "github.com/brendoncarroll/webfs/pkg/cells/rwacryptocell"
	"github.com/brendoncarroll/webfs/pkg/cells/secretboxcell"
	"github.com/brendoncarroll/webfs/pkg/webfsim"
)

func cellSpec2Model(x interface{}) (*webfsim.CellSpec, error) {
	var cellSpec *webfsim.CellSpec
	switch x2 := x.(type) {
	case *rwacryptocell.Spec:
		innerSpec, err := cellSpec2Model(x2.Inner)
		if err != nil {
			return nil, err
		}
		cellSpec = &webfsim.CellSpec{
			Spec: &webfsim.CellSpec_Rwacrypto{
				Rwacrypto: &webfsim.RWACryptoCellSpec{
					Inner:         innerSpec,
					Who:           x2.Who,
					PublicEntity:  x2.PublicEntity,
					PrivateEntity: x2.PrivateEntity,
				},
			},
		}
	case *secretboxcell.Spec:
		innerSpec, err := cellSpec2Model(x2.Inner)
		if err != nil {
			return nil, err
		}
		cellSpec = &webfsim.CellSpec{
			Spec: &webfsim.CellSpec_Secretbox{
				Secretbox: &webfsim.SecretBoxCellSpec{
					Inner:  innerSpec,
					Secret: x2.Secret,
				},
			},
		}
	case *httpcell.Spec:
		cellSpec = &webfsim.CellSpec{
			Spec: &webfsim.CellSpec_Http{
				Http: &webfsim.HTTPCellSpec{
					Url:     x2.URL,
					Headers: x2.Headers,
				},
			},
		}
	default:
		return nil, fmt.Errorf("unrecognized cell spec. type=%T", x)
	}

	return cellSpec, nil
}

func model2Cell(x *webfsim.CellSpec) (cells.Cell, error) {
	var cell cells.Cell

	switch x2 := x.Spec.(type) {
	case *webfsim.CellSpec_Http:
		spec := httpcell.Spec{
			URL:     x2.Http.Url,
			Headers: x2.Http.Headers,
		}
		cell = httpcell.New(spec)

	case *webfsim.CellSpec_Rwacrypto:
		innerCell, err := model2Cell(x2.Rwacrypto.Inner)
		if err != nil {
			return nil, err
		}
		spec := rwacryptocell.Spec{
			Inner: innerCell,

			Who:           x2.Rwacrypto.Who,
			PrivateEntity: x2.Rwacrypto.PrivateEntity,
			PublicEntity:  x2.Rwacrypto.PublicEntity,
		}
		cell = rwacryptocell.New(spec)

	case *webfsim.CellSpec_Secretbox:
		innerCell, err := model2Cell(x2.Secretbox.Inner)
		if err != nil {
			return nil, err
		}
		spec := secretboxcell.Spec{
			Inner: innerCell,
		}
		cell = secretboxcell.New(spec)
	default:
		return nil, fmt.Errorf("unrecognized cell spec. type=%T", x)
	}

	return cell, nil
}
