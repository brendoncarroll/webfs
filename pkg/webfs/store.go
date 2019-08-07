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
	return webref.Post(ctx, s.ms, s.opts, data)
}

func (s *Store) Get(ctx context.Context, ref Ref) ([]byte, error) {
	return webref.Get(ctx, s.ms, ref)
}

func (s *Store) MaxBlobSize() int {
	return s.ms.MaxBlobSize()
}

func (s *Store) Check(ctx context.Context, ref Ref) (bool, error) {
	panic("not implemented")
}

func (s *Store) Options() *webref.Options {
	return &s.opts
}
