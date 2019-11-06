package webfs

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/webfsim"
)

// AuxState for cells
type auxState struct {
	v *Volume
}

func (c *auxState) Get(ctx context.Context) ([]byte, error) {
	return c.v.spec.AuxState, nil
}

func (c *auxState) CAS(ctx context.Context, cur, next []byte) (bool, error) {
	err := c.v.ApplySpec(ctx, func(x webfsim.VolumeSpec) webfsim.VolumeSpec {
		y := x
		y.AuxState = next
		return y
	})

	return err == nil, err
}

func (c *auxState) URL() string {
	return c.v.spec.Id
}
