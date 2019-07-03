package webref

import (
	"context"
	"errors"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

type MuxStore struct {
	stores map[string]stores.Read
}

func NewMuxStore() *MuxStore {
	ms := &MuxStore{
		stores: make(map[string]stores.Read),
	}
	ms.stores["bc"] = stores.NewCAHttp("http://127.0.0.1:6667")
	return ms
}

func (ms *MuxStore) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	parts := strings.SplitN(prefix, "://", 2)
	if len(parts) < 2 {
		return "", errors.New("Can't resolve url: " + prefix)
	}
	scheme := parts[0]
	prefix = parts[1]
	s := ms.stores[scheme]
	key, err := s.(stores.WriteOnce).Post(ctx, prefix, data)
	if err != nil {
		return "", nil
	}
	return scheme + "://" + key, nil
}

func (ms *MuxStore) MaxBlobSize() int {
	min := 0
	for _, s := range ms.stores {
		wstore, ok := s.(stores.WriteOnce)
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
	parts := strings.SplitN(key, "://", 2)
	if len(parts) < 2 {
		return nil, errors.New("Can't resolve url: " + string(key))
	}
	scheme := string(parts[0])
	key2 := parts[1]
	s := ms.stores[scheme]
	return s.Get(ctx, key2)
}

func (ms *MuxStore) Check(ctx context.Context, key string) (bool, error) {
	parts := strings.SplitN(key, "://", 2)
	if len(parts) < 2 {
		return false, errors.New("Can't resolve url: " + key)
	}
	scheme := parts[0]
	key2 := parts[1]
	return ms.stores[scheme].Check(ctx, key2)
}
