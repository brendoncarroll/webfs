package webref

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

type Ref RepPackRef

func (r *Ref) Deref(ctx context.Context, s stores.Read) ([]byte, error) {
	return (*RepPackRef)(r).Deref(ctx, s)
}

func Post(ctx context.Context, s stores.WriteOnce, data []byte, opts Options) (*Ref, error) {
	r, err := PostRepPack(ctx, s, data, opts)
	return (*Ref)(r), err
}
