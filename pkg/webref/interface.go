package webref

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

func Get(ctx context.Context, s stores.Read, ref Ref) ([]byte, error) {
	return GetRepPack(ctx, s, ref.Ref)
}

func Post(ctx context.Context, s stores.WriteOnce, opts Options, data []byte) (*Ref, error) {
	r, err := PostRepPack(ctx, s, opts, data)
	if err != nil {
		return nil, err
	}
	return &Ref{
		Ref:   r,
		Attrs: nil,
	}, nil
}

func GetURLs(ref *Ref) []string {
	return ref.Ref.GetURLs()
}
