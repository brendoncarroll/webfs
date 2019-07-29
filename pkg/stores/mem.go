package stores

import (
	"context"
	"encoding/base64"
	"sync"

	"golang.org/x/crypto/sha3"
)

type MemStore struct {
	m sync.Map
}

func NewMemStore() *MemStore {
	return &MemStore{}
}

func (s *MemStore) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	dataStr := string(data)

	h := sha3.Sum256(data)
	key := prefix + base64.URLEncoding.EncodeToString(h[:])
	s.m.Store(key, dataStr)
	return key, nil
}

func (s *MemStore) Get(ctx context.Context, key string) ([]byte, error) {
	x, exists := s.m.Load(key)
	if !exists {
		return nil, nil
	}
	return []byte(x.(string)), nil
}

func (s *MemStore) MaxBlobSize() int {
	return 1 << 16
}
