package httpstore

import (
	"context"
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpStore(t *testing.T) {
	ctx := context.TODO()
	s, err := NewServer("127.0.0.1:", 4096)
	require.Nil(t, err)

	c := New(s.GetURL(), nil)
	err = c.Init(ctx)
	require.Nil(t, err)

	data := make([]byte, 1024)
	mrand.Read(data)

	key, err := c.Post(ctx, "", data)
	require.Nil(t, err)

	data2, err := c.Get(ctx, key)

	require.Nil(t, err)
	assert.Equal(t, data, data2)
}
