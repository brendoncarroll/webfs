package webref

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

func Get(ctx context.Context, s stores.Read, ref Ref) ([]byte, error) {
	return GetRepPack(ctx, s, ref.Ref)
}

func Post(ctx context.Context, s stores.Post, opts Options, data []byte) (*Ref, error) {
	r, err := PostRepPack(ctx, s, opts, data)
	if err != nil {
		return nil, err
	}
	return &Ref{
		Ref:   r,
		Attrs: nil,
	}, nil
}

func Delete(ctx context.Context, s stores.Delete, ref Ref) error {
	return DeleteRepPack(ctx, s, ref.Ref)
}

func GetURLs(ref *Ref) []string {
	return ref.Ref.GetURLs()
}
