package cells

import "context"

type Cell interface {
	ID() string
	Load(ctx context.Context) ([]byte, error)
}

type CASCell interface {
	Cell
	CAS(ctx context.Context, cur, next []byte) (bool, error)
}
