package gotcells

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/brendoncarroll/go-state/cadata"
	"github.com/brendoncarroll/go-state/cells"
	"github.com/gotvc/got/pkg/gotvc"
)

type BranchCell struct {
	inner   cells.Cell
	gotvc   *gotvc.Operator
	vcStore cadata.Store
}

func NewBranch(inner cells.Cell, vcop *gotvc.Operator, vcStore cadata.Store) *BranchCell {
	return &BranchCell{
		inner:   inner,
		gotvc:   vcop,
		vcStore: vcStore,
	}
}

func (c *BranchCell) Read(ctx context.Context, buf []byte) (int, error) {
	n, err := c.inner.Read(ctx, buf)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return n, nil
	}
	var snap gotvc.Snap
	if err := json.Unmarshal(buf[:n], &snap); err != nil {
		return 0, err
	}
	data, err := json.Marshal(snap.Root)
	if err != nil {
		return 0, err
	}
	return copy(buf, data), nil
}

func (c *BranchCell) CAS(ctx context.Context, actual, prev, next []byte) (bool, int, error) {
	return false, 0, errors.New("writing to got branches not yet supported")
}

func (c *BranchCell) MaxSize() int {
	return 1 << 10
}
