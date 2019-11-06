package webref

import (
	"context"

	"github.com/golang/protobuf/proto"
	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/crypto/sha3"
)

type Cache struct {
	slow  Getter
	cache *lru.ARCCache
}

func NewCache(inner Getter) *Cache {
	cache, err := lru.NewARC(128)
	if err != nil {
		panic(err)
	}
	return &Cache{
		slow:  inner,
		cache: cache,
	}
}

func (c *Cache) put(key [32]byte, data []byte) {
	data2 := make([]byte, len(data))
	copy(data2, data)
	c.cache.Add(key, data)
}

func (c *Cache) Get(ctx context.Context, ref *Ref) ([]byte, error) {
	key := refKey(ref)
	v, ok := c.cache.Get(key)
	if ok {
		return v.([]byte), nil
	}

	data, err := c.slow.Get(ctx, ref)
	if err != nil {
		return nil, err
	}
	c.put(key, data)
	return data, nil
}

func refKey(r *Ref) [32]byte {
	data, err := proto.Marshal(r)
	if err != nil {
		panic(err)
	}
	return sha3.Sum256(data)
}
