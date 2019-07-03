package webfs

import (
	"context"

	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Ref = webref.Ref
type WriteOnce = webref.WriteOnce
type Read = webref.Read
type ReadWriteOnce = webref.ReadWriteOnce

type Store struct {
	ms   *webref.MuxStore
	opts webref.Options
}

func NewStore() *Store {
	return &Store{
		ms: webref.NewMuxStore(),
		opts: webref.Options{
			Replicas: map[string]int{
				"bc://": 1,
			},
		},
	}
}

func (s *Store) Post(ctx context.Context, data []byte) (*Ref, error) {
	return webref.PostRepPack(ctx, s.ms, data, s.opts)
}

func (s *Store) Get(ctx context.Context, ref Ref) ([]byte, error) {
	return ref.Deref(ctx, s.ms)
}

func (s *Store) MaxBlobSize() int {
	return s.ms.MaxBlobSize()
}

func (s *Store) Check(ctx context.Context, key string) (bool, error) {
	return s.ms.Check(ctx, key)
}
