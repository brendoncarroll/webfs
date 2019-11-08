package sftpstore

import "context"

const MaxBlobSize = 1 << 16

type Spec struct {
}

type SFTPStore struct {
	spec Spec
}

func New(spec Spec) *SFTPStore {
	return &SFTPStore{spec: spec}
}

func (s *SFTPStore) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}

func (s *SFTPStore) Post(ctx context.Context, key string, data []byte) (string, error) {
	return "", nil
}

func (s *SFTPStore) Delete(ctx context.Context, key string) error {
	return nil
}

func (s *SFTPStore) MaxBlobSize() int {
	return MaxBlobSize
}
