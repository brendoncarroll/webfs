package gdatcache

import (
	"context"

	"blobcache.io/blobcache/src/bcsdk"
	"github.com/gotvc/got/src/gdat"
	lru "github.com/hashicorp/golang-lru/v2"
)

type Cache[T any] struct {
	mach    *gdat.Machine
	marshal bcsdk.Marshaler[T]
	parse   bcsdk.Parser[T]
	c       lru.Cache[gdat.Ref, T]
}

func New[T any](mach *gdat.Machine, marshal bcsdk.Marshaler[T], parser bcsdk.Parser[T], size int) *Cache[T] {
	c, _ := lru.New[gdat.Ref, T](size)
	return &Cache[T]{
		mach:    mach,
		marshal: marshal,
		parse:   parser,
		c:       *c,
	}
}

func (c *Cache[T]) Post(ctx context.Context, s bcsdk.WO, x T) (gdat.Ref, error) {
	return c.mach.Post(ctx, s, c.marshal(x, nil))
}

func (c *Cache[T]) Get(ctx context.Context, s bcsdk.RO, ref gdat.Ref) (T, error) {
	ret, ok := c.c.Get(ref)
	if ok {
		return ret, nil
	}
	if err := c.mach.GetF(ctx, s, ref, func(data []byte) error {
		var err error
		ret, err = c.parse(data)
		return err
	}); err != nil {
		return ret, err
	}
	c.c.Add(ref, ret)
	return ret, nil
}
