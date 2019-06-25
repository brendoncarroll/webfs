package l2

import "context"

type Read interface {
	Get(ctx context.Context, ref Ref) (data []byte, err error)
}

type WriteOnce interface {
	Post(ctx context.Context, data []byte) (*Ref, error)
	//PostBatch(ctx context.Context, data [][]byte) ([]Ref, error)
	MaxBlobSize() int
}

type ReadWriteOnce interface {
	Read
	WriteOnce
}
