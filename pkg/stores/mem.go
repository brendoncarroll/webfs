package stores

import (
	"context"
	"sync"
)

type MemStore struct {
	m sync.Map
}

func (s *MemStore) Post(ctx context.Context, key string, data []byte) (string, error) {
	s.m.Store(key, data)
	return key, nil
}

func (s *MemStore) Get(ctx context.Context, key string) ([]byte, error) {
	x, exists := s.m.Load(key)
	if !exists {
		return nil, nil
	}
	return x.([]byte), nil
}

func (s *MemStore) MaxBlobSize() int {
	return 1 << 16
}
