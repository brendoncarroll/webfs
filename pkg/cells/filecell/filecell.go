package filecell

import (
	"bytes"
	"context"
	"encoding/json"
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

func (c *Cell) Get(ctx context.Context) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return ioutil.ReadFile(c.p)
}

func (c *Cell) CAS(ctx context.Context, cur, next []byte) (bool, error) {
	next = reformat(next)
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := ioutil.ReadFile(c.p)
	if err != nil {
		return false, err
	}

	if bytes.Compare(cur, data) == 0 {
		if err := ioutil.WriteFile(c.p, next, 0644); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

func reformat(x []byte) []byte {
	var v map[string]interface{}
	if err := json.Unmarshal(x, &v); err != nil {
		panic(err)
		return x
	}
	y, _ := json.MarshalIndent(v, "", "  ")
	return y
}
