package stores

import (
	"context"
	"errors"
)

var (
	ErrNotFound        = errors.New("blob not found")
	ErrMaxSizeExceeded = errors.New("max blob size exceeded")
)

type Read interface {
	Get(ctx context.Context, k string) ([]byte, error)
}

type Check interface {
	Check(ctx context.Context, k string) (bool, error)
}

type WriteOnce interface {
	Post(ctx context.Context, prefix string, data []byte) (string, error)
	MaxBlobSize() int
}

type ReadWriteOnce interface {
	Read
	WriteOnce
}
