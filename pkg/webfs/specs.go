package webfs

import (
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/httpcell"
	cryptocell "github.com/brendoncarroll/webfs/pkg/cells/rwacryptocell"
	rwacryptocell "github.com/brendoncarroll/webfs/pkg/cells/rwacryptocell"
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
			Spec: &webfsim.CellSpec_Crypto{
				Crypto: &webfsim.CryptoCellSpec{
					Inner:         innerSpec,
					Who:           x2.Who,
					PublicEntity:  x2.PublicEntity,
					PrivateEntity: x2.PrivateEntity,
				},
			},
		}
	case *httpcell.Spec:
		cellSpec = &webfsim.CellSpec{
			Spec: &webfsim.CellSpec_Http{
				Http: &webfsim.HTTPCellSpec{
					Url:        x2.URL,
					AuthHeader: x2.AuthHeader,
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
			URL:        x2.Http.Url,
			AuthHeader: x2.Http.AuthHeader,
		}
		cell = httpcell.New(spec)

	case *webfsim.CellSpec_Crypto:
		innerCell, err := model2Cell(x2.Crypto.Inner)
		if err != nil {
			return nil, err
		}
		spec := rwacryptocell.Spec{
			Inner: innerCell,

			Who:           x2.Crypto.Who,
			PrivateEntity: x2.Crypto.PrivateEntity,
			PublicEntity:  x2.Crypto.PublicEntity,
		}
		cell = cryptocell.New(spec)

	default:
		return nil, fmt.Errorf("unrecognized cell spec. type=%T", x)
	}

	return cell, nil
}
