package webfs

import (
	"context"
	"errors"

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
	parent := c.v.parent
	if parent == nil {
		return false, errors.New("volume is root")
	}
	nameInParent := c.v.nameInParent

	prev := c.v.spec
	var err error

	switch p2 := parent.(type) {
	case *Volume:
		err = p2.put(ctx, func(o *webfsim.Object) (*webfsim.Object, error) {
			o2 := &webfsim.Object{
				Value: &webfsim.Object_Volume{
					&webfsim.VolumeSpec{
						Id:       prev.Id,
						CellSpec: prev.CellSpec,
						AuxState: next,
					},
				},
			}
			return o2, nil
		})
	case *Dir:
		err = p2.put(ctx, nameInParent, func(o *webfsim.Object) (*webfsim.Object, error) {
			o2 := &webfsim.Object{
				Value: &webfsim.Object_Volume{
					&webfsim.VolumeSpec{
						Id:       prev.Id,
						CellSpec: prev.CellSpec,
						AuxState: next,
					},
				},
			}
			return o2, nil
		})

	case *File:
		panic("parent is file")
	}

	return true, err
}

func (c *auxState) URL() string {
	return c.v.spec.Id
}
