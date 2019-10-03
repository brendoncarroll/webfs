package cells

import "context"

type Cell interface {
	URL() string
	Get(ctx context.Context) ([]byte, error)
	CAS(ctx context.Context, cur, next []byte) (bool, error)
}

type StatefulCell interface {
	Cell
	AuxState() string
}
