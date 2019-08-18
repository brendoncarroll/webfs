package stores

import (
	"context"
	"encoding/base64"
	"sync"

	"golang.org/x/crypto/sha3"
)

type MemStore struct {
	m           sync.Map
	maxBlobSize int
}

func NewMemStore(maxBlobSize int) *MemStore {
	return &MemStore{
		maxBlobSize: maxBlobSize,
	}
}

func (s *MemStore) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	data2 := make([]byte, len(data))
	copy(data2, data)

	h := sha3.Sum256(data)
	key := prefix + base64.URLEncoding.EncodeToString(h[:])
	s.m.Store(key, data2)
	return key, nil
}

func (s *MemStore) Get(ctx context.Context, key string) ([]byte, error) {
	x, exists := s.m.Load(key)
	if !exists {
		return nil, nil
	}
	x2 := x.([]byte)
	data := make([]byte, len(x2))
	copy(data, x2)
	return data, nil
}

func (s *MemStore) MaxBlobSize() int {
	return s.maxBlobSize
}
