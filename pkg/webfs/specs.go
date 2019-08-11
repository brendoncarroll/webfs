package webfs

import (
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/cryptocell"
	"github.com/brendoncarroll/webfs/pkg/cells/httpcell"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
)

func cellSpec2Model(x interface{}) (*models.CellSpec, error) {
	var cellSpec *models.CellSpec
	switch x2 := x.(type) {
	case *cryptocell.Spec:
		innerSpec, err := cellSpec2Model(x2.Inner)
		if err != nil {
			return nil, err
		}
		cellSpec = &models.CellSpec{
			Spec: &models.CellSpec_Crypto{
				Crypto: &models.CryptoCellSpec{
					Inner:         innerSpec,
					Who:           x2.Who,
					PublicEntity:  x2.PublicEntity,
					PrivateEntity: x2.PrivateEntity,
				},
			},
		}
	case *httpcell.Spec:
		cellSpec = &models.CellSpec{
			Spec: &models.CellSpec_Http{
				Http: &models.HTTPCellSpec{
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

func model2Cell(x *models.CellSpec) (cells.Cell, error) {
	var cell cells.Cell

	switch x2 := x.Spec.(type) {
	case *models.CellSpec_Http:
		spec := httpcell.Spec{
			URL:        x2.Http.Url,
			AuthHeader: x2.Http.AuthHeader,
		}
		cell = httpcell.New(spec)

	case *models.CellSpec_Crypto:
		innerCell, err := model2Cell(x2.Crypto.Inner)
		if err != nil {
			return nil, err
		}
		spec := cryptocell.Spec{
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
