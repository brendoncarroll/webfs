package filecell

import (
	"context"
	"io/ioutil"
	"sync"

	"github.com/brendoncarroll/webfs/pkg/cells"
)

func init() {
	cells.Register(Spec{}, func(x interface{}) cells.Cell {
		return New(x.(Spec).Path)
	})
}

type Spec struct {
	Path string
}

type Cell struct {
	mu sync.Mutex
	p  string
}

func New(p string) *Cell {
	c := &Cell{p: p}
	return c
}

func (c *Cell) ID() string {
	return "filecell-%s" + c.p
}

func (c *Cell) Load(ctx context.Context) ([]byte, error) {
	return ioutil.ReadFile(c.p)
}

func (c *Cell) CAS(ctx context.Context, cur, next []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return ioutil.WriteFile(c.p, next, 0644)
}
