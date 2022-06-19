package webfs

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPotConfigPaths(t *testing.T) {
	pcps := potConfigPaths("a/b/c/d")
	require.Equal(t, []string{
		"a.webfs",
		"a/b.webfs",
		"a/b/c.webfs",
		"a/b/c/d.webfs",
	}, pcps)

	pcps = potConfigPaths("a")
	require.Equal(t, []string{"a.webfs"}, pcps)

	pcps = potConfigPaths("")
	require.ElementsMatch(t, []string{}, pcps)
}

func TestPut(t *testing.T) {
	ctx := context.Background()
	wfs := newTestWebFS(t)
	require.NoError(t, wfs.PutFile(ctx, "test", strings.NewReader("my test data")))
}

func TestPutCat(t *testing.T) {
	ctx := context.Background()
	wfs := newTestWebFS(t)
	testData := "my test data"
	require.NoError(t, wfs.PutFile(ctx, "test", strings.NewReader(testData)))
	buf := &bytes.Buffer{}
	require.NoError(t, wfs.Cat(ctx, "test", buf))
	require.Equal(t, testData, buf.String())
}

func newTestWebFS(t testing.TB) *FS {
	fs, err := New(VolumeSpec{
		Cell:  CellSpec{Memory: &struct{}{}},
		Store: StoreSpec{Memory: &struct{}{}},
	})
	require.NoError(t, err)
	return fs
}
