package webref

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

func GetSingle(ctx context.Context, s stores.Read, u string) ([]byte, error) {
	data, err := s.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func DeleteSingle(ctx context.Context, s stores.Delete, u string) error {
	return s.Delete(ctx, u)
}
