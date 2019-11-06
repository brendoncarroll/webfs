package cells

import (
	"context"
	"errors"
)

var _ Cell = ZeroCell{}

type ZeroCell struct {
}

func (c ZeroCell) Get(ctx context.Context) ([]byte, error) {
	return nil, nil
}

func (c ZeroCell) CAS(ctx context.Context, cur, next []byte) (bool, error) {
	return false, errors.New("cannot write to zero cell")
}

func (c ZeroCell) URL() string {
	return ""
}
