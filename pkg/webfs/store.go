package webfs

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Ref = webref.Ref
type WriteOnce = webref.WriteOnce
type Read = webref.Read
type ReadWriteOnce = webref.ReadWriteOnce

type Store struct {
	ms   stores.ReadWriteOnce
	opts webref.Options
}

func (s *Store) Post(ctx context.Context, data []byte) (*Ref, error) {
	return webref.Post(ctx, s.ms, data, s.opts)
}

func (s *Store) Get(ctx context.Context, ref Ref) ([]byte, error) {
	return ref.Deref(ctx, s.ms)
}

func (s *Store) MaxBlobSize() int {
	return s.ms.MaxBlobSize()
}

func (s *Store) Check(ctx context.Context, key string) (bool, error) {
	return s.ms.(stores.Check).Check(ctx, key)
}
