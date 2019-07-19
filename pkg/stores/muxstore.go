package stores

import (
	"context"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/stores/cahttp"
	"github.com/brendoncarroll/webfs/pkg/stores/ipfsstore"
)

type StoreRoute struct {
	Prefix string
	Store  Read
}

type MuxStore struct {
	routes []StoreRoute
}

func NewMuxStore() *MuxStore {
	ms := &MuxStore{
		routes: []StoreRoute{
			{
				Prefix: "bc://",
				Store:  cahttp.NewCAHttp("http://127.0.0.1:6667", "bc://"),
			},
			{
				Prefix: "ipfs://",
				Store:  ipfsstore.New("localhost:5001"),
			},
		},
	}

	return ms
}

func (ms *MuxStore) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	return ms.getStore(prefix).(WriteOnce).Post(ctx, prefix, data)
}

func (ms *MuxStore) MaxBlobSize() int {
	min := 0
	for _, route := range ms.routes {
		wstore, ok := route.Store.(WriteOnce)
		if ok {
			max := wstore.MaxBlobSize()
			if max < min || min == 0 {
				min = max
			}
		}
	}
	return min
}

func (ms *MuxStore) Get(ctx context.Context, key string) ([]byte, error) {
	return ms.getStore(key).Get(ctx, key)
}

func (ms *MuxStore) Check(ctx context.Context, key string) (bool, error) {
	return ms.getStore(key).(Check).Check(ctx, key)
}

func (ms *MuxStore) getStore(x string) Read {
	for _, route := range ms.routes {
		if strings.HasPrefix(x, route.Prefix) {
			return route.Store
		}
	}
	return nil
}
