package webref

import (
	"context"
)

type Read interface {
	Get(context.Context, Ref) ([]byte, error)
}

type WriteOnce interface {
	Post(ctx context.Context, data []byte) (*Ref, error)
	MaxBlobSize() int
}

type ReadWriteOnce interface {
	Read
	WriteOnce
}
