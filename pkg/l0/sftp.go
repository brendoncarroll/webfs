package l0

import "context"

type SFTPStore struct {
}

func NewSFTPStore() *SFTPStore {
	return &SFTPStore{}
}

func (s *SFTPStore) Put(ctx context.Context, key string, data []byte) error {
	return nil
}

func (s *SFTPStore) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}
