package webfs

import (
	"context"
	"errors"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/l0"
	"github.com/brendoncarroll/webfs/pkg/l2"
)

type Ref = l2.Ref
type WriteOnce = l2.WriteOnce
type Read = l2.Read
type ReadWriteOnce = l2.ReadWriteOnce

type MuxStore struct {
	stores map[string]l0.Read
}

func NewMuxStore() *MuxStore {
	ms := &MuxStore{
		stores: make(map[string]l0.Read),
	}
	ms.stores["bc"] = l0.NewCAHttp("http://127.0.0.1:6667")
	return ms
}

func (ms *MuxStore) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	parts := strings.SplitN(prefix, "://", 2)
	if len(parts) < 2 {
		return "", errors.New("Can't resolve url: " + prefix)
	}
	scheme := parts[0]
	prefix = parts[1]
	s := ms.stores[scheme]
	key, err := s.(l0.WriteOnce).Post(ctx, prefix, data)
	if err != nil {
		return "", nil
	}
	return scheme + "://" + key, nil
}

func (ms *MuxStore) MaxBlobSize() int {
	min := 0
	for _, s := range ms.stores {
		wstore, ok := s.(l0.WriteOnce)
		if ok {
			max := wstore.MaxBlobSize()
			if max < min || min == 0 {
				min = max
			}
		}
	}
	return min
}

func (ms *MuxStore) Get(ctx context.Context, key string) ([]byte, error) {
	parts := strings.SplitN(key, "://", 2)
	if len(parts) < 2 {
		return nil, errors.New("Can't resolve url: " + string(key))
	}
	scheme := string(parts[0])
	key2 := parts[1]
	s := ms.stores[scheme]
	return s.Get(ctx, key2)
}

func (ms *MuxStore) Check(ctx context.Context, key string) (bool, error) {
	parts := strings.SplitN(key, "://", 2)
	if len(parts) < 2 {
		return false, errors.New("Can't resolve url: " + key)
	}
	scheme := parts[0]
	key2 := parts[1]
	return ms.stores[scheme].Check(ctx, key2)
}

type Store struct {
	ms     *MuxStore
	l2opts l2.Options
}

func NewStore() *Store {
	return &Store{
		ms: NewMuxStore(),
		l2opts: l2.Options{
			Replicas: map[string]int{
				"bc://": 1,
			},
		},
	}
}

func (s *Store) Post(ctx context.Context, data []byte) (*Ref, error) {
	return l2.Post(ctx, s.ms, data, s.l2opts)
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
