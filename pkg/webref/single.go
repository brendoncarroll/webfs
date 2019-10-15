package webref

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

func PostSingle(ctx context.Context, s stores.Post, prefix string, data []byte, o Options) (*Ref, error) {
	u, err := s.Post(ctx, prefix, data)
	if err != nil {
		return nil, err
	}
	return &Ref{
		Ref: &Ref_Url{u},
	}, nil
}

func GetSingle(ctx context.Context, s stores.Read, u string) ([]byte, error) {
	data, err := s.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func CheckSingle(ctx context.Context, s stores.Check, u string) []RefStatus {
	err := s.Check(ctx, u)
	return []RefStatus{{URL: u, Error: err}}
}

func DeleteSingle(ctx context.Context, s stores.Delete, u string) error {
	return s.Delete(ctx, u)
}
