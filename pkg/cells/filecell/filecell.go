package filecell

import (
	"bytes"
	"context"
	"io/ioutil"
	"sync"

	"github.com/brendoncarroll/go-state/posixfs"
)

type Cell struct {
	mu sync.Mutex
	fs posixfs.FS
	p  string
}

func New(pfs posixfs.FS, p string) *Cell {
	c := &Cell{fs: pfs, p: p}
	return c
}

func (c *Cell) Read(ctx context.Context, buf []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := posixfs.ReadFile(ctx, c.fs, c.p)
	if err != nil && !posixfs.IsErrNotExist(err) {
		return 0, err
	}
	return copy(buf, data), nil
}

func (c *Cell) CAS(ctx context.Context, actual, prev, next []byte) (bool, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := posixfs.ReadFile(ctx, c.fs, c.p)
	if err != nil && !posixfs.IsErrNotExist(err) {
		return false, 0, err
	}
	var swapped bool
	if bytes.Equal(prev, data) {
		if err := ioutil.WriteFile(c.p, next, 0644); err != nil {
			return false, 0, err
		}
		swapped = true
		data = next
	} else {
		swapped = false
	}
	return swapped, copy(actual, data), nil
}

func (c *Cell) MaxSize() int {
	return 1 << 16
}
