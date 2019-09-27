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
	Check(ctx context.Context, k string) error
}

type Post interface {
	Post(ctx context.Context, prefix string, data []byte) (string, error)
	MaxBlobSize() int
}

type Delete interface {
	Delete(ctx context.Context, key string) error
}

type ReadPost interface {
	Read
	Post
}

type ReadPostDelete interface {
	Read
	Post
	Delete
}
