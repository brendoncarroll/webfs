package httpcell

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/brendoncarroll/webfs/pkg/cells"

	"github.com/stretchr/testify/assert"
)

func TestHttpCell(t *testing.T) {
	ctx := context.TODO()
	ctx, cf := context.WithCancel(ctx)
	defer cf()

	const addr = "127.0.0.1:"

	server := NewServer()
	go server.Serve(ctx, addr)
	server.newCell("cell1")

	u := server.URL() + "cell1"
	cell := New(Spec{URL: u})

	data, err := cell.Get(ctx)
	assert.Nil(t, err)
	assert.Len(t, data, 0)

	testData := []string{
		"my test string",
		"the second string",
		"another one",
	}
	for _, s := range testData {
		next := []byte(s)
		cur := data
		success, err := cell.CAS(ctx, cur, next)
		assert.Nil(t, err)
		assert.True(t, success)
		if !success {
			break
		}
		data = next
	}
}

func TestSuite(t *testing.T) {
	ctx := context.TODO()
	ctx, cf := context.WithCancel(ctx)
	defer cf()

	const addr = "127.0.0.1:"
	server := NewServer()
	go server.Serve(ctx, addr)

	cells.CellTestSuite(t, func() cells.Cell {
		n := rand.Int()
		name := fmt.Sprint("cell-", n)
		server.newCell(name)

		u := server.URL() + name
		cell := New(Spec{URL: u})

		return cell
	})
}
