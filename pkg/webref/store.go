package webref

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

type Read interface {
	Get(context.Context, Ref) ([]byte, error)
}

type WriteOnce interface {
	Post(ctx context.Context, data []byte) (*Ref, error)
	MaxBlobSize() int
}

type ReadWriteOnce interface {
	Read
	WriteOnce
}

type storeWrapper struct {
	s stores.ReadWriteOnce
}

func (s *storeWrapper) Post(ctx context.Context, data []byte) (*Ref, error) {
	return Post(ctx, s.s, data, DefaultOptions())
}

func (s *storeWrapper) Get(ctx context.Context, ref Ref) ([]byte, error) {
	return Get(ctx, s.s, ref)
}

func (s *storeWrapper) MaxBlobSize() int {
	return s.MaxBlobSize()
}

func NewMemStore() ReadWriteOnce {
	return &storeWrapper{s: stores.NewMemStore()}
}
