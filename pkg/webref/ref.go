package webref

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

type Ref RepPackRef

func Get(ctx context.Context, s stores.Read, ref Ref) ([]byte, error) {
	return GetRepPack(ctx, s, RepPackRef(ref))
}

func Post(ctx context.Context, s stores.WriteOnce, data []byte, opts Options) (*Ref, error) {
	r, err := PostRepPack(ctx, s, data, opts)
	return (*Ref)(r), err
}
