package webfs

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"testing"

	"golang.org/x/crypto/sha3"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/memcell"
	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = context.TODO()

func getTestFS() *WebFS {
	ms := stores.NewMemStore()
	mc := cells.Make(memcell.Spec{})
	wfs, err := New(mc, ms)
	if err != nil {
		panic(err)
	}
	return wfs
}

func TestFind(t *testing.T) {
	wfs := getTestFS()

	err := wfs.Mkdir(ctx, "")
	require.Nil(t, err)

	objs, err := wfs.Find(ctx, "")
	require.Nil(t, err)
	if assert.Len(t, objs, 2) {
		_, ok := objs[1].(*Dir)
		assert.True(t, ok)
		_, ok = objs[0].(*Volume)
		assert.True(t, ok)
	}
}

func TestMkdir(t *testing.T) {
	dirpaths := []string{
		"",
		"testdir1",
		"testdir2",
		"testdir2/testdir2.1",
	}
	wfs := getTestFS()

	for _, dp := range dirpaths {
		err := wfs.Mkdir(ctx, dp)
		assert.Nil(t, err)
	}

	for _, dp := range dirpaths {
		o, err := wfs.Lookup(ctx, dp)
		require.Nil(t, err)
		require.NotNil(t, o)
		_, ok := o.(*Dir)
		assert.True(t, ok)
	}
}

func TestFileWriteRead(t *testing.T) {
	const N = 3
	wfs := getTestFS()
	err := wfs.Mkdir(ctx, "")
	require.Nil(t, err)

	files := map[string][32]byte{}
	for i := 0; i < N; i++ {
		filePath := fmt.Sprintf("file%03d", i)
		data := make([]byte, 1<<20)
		_, err := rand.Read(data)
		assert.Nil(t, err)
		files[filePath] = sha3.Sum256(data)

		err = wfs.ImportFile(ctx, bytes.NewBuffer(data), filePath)
		assert.Nil(t, err)
	}

	for filePath, fileHash := range files {
		fh, err := wfs.OpenFile(ctx, filePath)
		require.Nil(t, err)
		data, err := ioutil.ReadAll(fh)
		require.Nil(t, err)

		expected := fileHash
		actual := sha3.Sum256(data)
		assert.Equal(t, expected, actual)
	}
}
