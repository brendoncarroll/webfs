package cells

import "context"

type Cell interface {
	URL() string
	Get(ctx context.Context) ([]byte, error)
	CAS(ctx context.Context, cur, next []byte) (bool, error)
}

type Claim interface {
	Claim(context.Context) error
}

type Join interface {
	Join(context.Context, func(x interface{}) bool) error
}

type Inspect interface {
	Inspect(context.Context) string
}

type Subscribe interface {
	Subscribe(chan []byte)
	Unsubscribe(chan []byte)
}
