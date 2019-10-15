package webref

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

func GetSlice(ctx context.Context, s stores.Read, ref *Slice) ([]byte, error) {
	panic("not implemented")
}

func DeleteSlice(ctx context.Context, s stores.Read, ref *Slice) error {
	panic("not implemented")
}

func CheckSlice(ctx context.Context, s stores.Read, ref *Slice) RefStatus {
	panic("not implemented")
}
