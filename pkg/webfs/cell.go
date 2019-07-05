package webfs

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/httpcell"

	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Cell = cells.Cell
type CASCell = cells.CASCell

type CellSpec struct {
	HTTPCell *httpcell.Spec
	// Do not add FileCell or MemCell here
}

type CellContents struct {
	ObjectRef webref.Ref `json:"object_ref"`
	Options   Options    `json:"options"`
}

func (cc *CellContents) Marshal() []byte {
	data, _ := json.Marshal(cc)
	return data
}

func (cc *CellContents) Unmarshal(data []byte) error {
	return json.Unmarshal(data, cc)
}

type CellMutator func(CellContents) (*CellContents, error)

func GetContents(ctx context.Context, cell Cell) (*CellContents, error) {
	data, err := cell.Load(ctx)
	if err != nil {
		return nil, err
	}
	cc := CellContents{}
	if err := cc.Unmarshal(data); err != nil {
		return nil, err
	}
	return &cc, nil
}

func Apply(ctx context.Context, cell Cell, f CellMutator) error {
	wcell, ok := cell.(CASCell)
	if !ok {
		return errors.New("cell is not writeable")
	}

	const maxRetries = 10
	success := false
	for i := 0; !success && i < maxRetries; i++ {
		current, err := wcell.Load(ctx)
		if err != nil {
			return err
		}
		currentC := CellContents{}
		if err := currentC.Unmarshal(current); err != nil {
			return err
		}
		nextC, err := f(currentC)
		if err != nil {
			return err
		}
		next := nextC.Marshal()
		success, err = wcell.CAS(ctx, current, next)
		if err != nil {
			return err
		}
	}

	if !success {
		return errors.New("could not complete CAS")
	}

	return nil
}
