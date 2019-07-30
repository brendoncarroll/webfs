package cells

import "context"

type Cell interface {
	ID() string
	Get(ctx context.Context) ([]byte, error)
	CAS(ctx context.Context, cur, next []byte) (bool, error)
}
