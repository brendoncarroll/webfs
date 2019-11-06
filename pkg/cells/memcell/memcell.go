package memcell

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

var count = int64(0)

type Spec struct{}

type Cell struct {
	n  uint
	mu sync.Mutex
	x  []byte
}

func New() *Cell {
	next := atomic.AddInt64(&count, 1)
	return &Cell{n: uint(next - 1)}
}

func (c *Cell) URL() string {
	return fmt.Sprintf("mem://%d", c.n)
}

func (c *Cell) CAS(ctx context.Context, cur, next []byte) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if bytes.Compare(c.x, cur) == 0 {
		next2 := make([]byte, len(next))
		copy(next2, next)
		c.x = next2
		return true, nil
	}
	return false, nil
}

func (c *Cell) Get(ctx context.Context) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	data := make([]byte, len(c.x))
	copy(data, c.x)
	return c.x, nil
}

func (c *Cell) GetOrDie() []byte {
	return c.x
}
