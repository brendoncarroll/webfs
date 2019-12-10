package webfs

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"testing"

	"golang.org/x/crypto/sha3"

	"github.com/brendoncarroll/webfs/pkg/cells/httpcell"
	"github.com/brendoncarroll/webfs/pkg/cells/memcell"
	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = context.TODO()

func getTestFS() *WebFS {
	ms := stores.NewMemStore(4096)
	mc := memcell.New()
	wfs, err := New(mc, ms)
	if err != nil {
		panic(err)
	}

	objs, err := wfs.GetAtPath(ctx, "")
	if err != nil {
		panic(err)
	}
	objs[0].(*Volume).ApplyOptions(ctx, func(x *Options) *Options {
		x.DataOpts.Replicas[""] = 1
		return x
	})
	return wfs
}

func TestFind(t *testing.T) {
	wfs := getTestFS()

	err := wfs.Mkdir(ctx, "")
	require.Nil(t, err)

	objs, err := wfs.GetAtPath(ctx, "")
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

func TestNewVolume(t *testing.T) {
	ctx, cf := context.WithCancel(ctx)
	defer cf()
	mc := memcell.New()
	ms := stores.NewMemStore(4096)
	// setup webfs
	wfs, err := New(mc, ms)
	require.Nil(t, err)
	rootV, err := wfs.GetVolume(ctx, RootVolumeID)
	require.Nil(t, err)
	err = rootV.ApplyOptions(ctx, func(x *Options) *Options {
		x.DataOpts.Replicas[""] = 1
		return x
	})
	require.Nil(t, err)

	// setup cell server, and client
	cellServer := httpcell.NewServer()
	cellName := "cell1"

	go func() {
		if err := cellServer.Serve(ctx, "127.0.0.1:"); err != nil {
			t.Log(err)
		}
	}()
	spec := cellServer.CreateCell(cellName)

	// create volume
	p := "/testvol"
	vs := webfsim.VolumeSpec{
		CellSpec: &webfsim.CellSpec{
			Spec: &webfsim.CellSpec_Http{
				Http: &webfsim.HTTPCellSpec{
					Url: spec.URL,
				},
			},
		},
	}
	err = wfs.Mkdir(ctx, "")
	require.Nil(t, err)
	vol, err := wfs.NewVolume(ctx, p)
	require.Nil(t, err)
	err = vol.ApplySpec(ctx, func(x webfsim.VolumeSpec) webfsim.VolumeSpec {
		return vs
	})
	require.Nil(t, err)
	err = vol.ApplyOptions(ctx, func(x *Options) *Options {
		x.DataOpts.Replicas[""] = 1
		return x
	})
	require.Nil(t, err)

	err = wfs.Mkdir(ctx, "/testvol")
	require.Nil(t, err)
	err = wfs.Mkdir(ctx, "/testvol/testdir")
	require.Nil(t, err)
	ents, err := wfs.Ls(ctx, "/testvol")
	require.Nil(t, err)
	assert.Len(t, ents, 1)
}

func TestMove(t *testing.T) {

	const (
		p1 = "testfile.txt"
		p2 = "testfile2.txt"
	)

	wfs := getTestFS()
	err := wfs.Mkdir(ctx, "")
	require.Nil(t, err)

	buf := bytes.NewBuffer([]byte("test file contents"))
	err = wfs.ImportFile(ctx, buf, p1)
	require.Nil(t, err)

	f, err := wfs.Lookup(ctx, p1)
	require.Nil(t, err)
	require.NotNil(t, f)

	err = wfs.Move(ctx, f, p2)
	require.Nil(t, err)

	_, err = wfs.Lookup(ctx, p1)
	require.Equal(t, ErrNotExist, err)

	f, err = wfs.Lookup(ctx, p2)
	require.Nil(t, err)
	require.NotNil(t, f)

	const (
		dp1 = "dir1"
		dp2 = "dir2"
	)
	err = wfs.Mkdir(ctx, dp1)
	require.Nil(t, err)
	dir, err := wfs.Lookup(ctx, dp1)
	require.Nil(t, err)

	err = wfs.Move(ctx, dir, dp2)
	require.Nil(t, err)

	_, err = wfs.Lookup(ctx, dp1)
	require.Equal(t, ErrNotExist, err)
	dir2, err := wfs.Lookup(ctx, dp2)
	require.Nil(t, err)
	require.NotNil(t, dir2)
}

func TestLookupParent(t *testing.T) {
	wfs := getTestFS()
	err := wfs.Mkdir(ctx, "")
	require.Nil(t, err)

	const p = "testfile.txt"
	err = wfs.ImportFile(ctx, nil, p)
	require.Nil(t, err)

	o, name, err := wfs.lookupParent(ctx, ParsePath(p))
	require.Nil(t, err)
	_, ok := o.(*Dir)
	if !ok {
		t.Error("did not find root dir")
	}
	require.Equal(t, p, name)

	v1, err := wfs.NewVolume(ctx, "vol1")
	require.Nil(t, err)

	o, name, err = wfs.lookupParent(ctx, ParsePath("vol1"))
	require.Nil(t, err)
	assert.Equal(t, v1.Describe(), o.Describe())
	assert.Equal(t, name, "")
}

func TestDeleteAt(t *testing.T) {
	wfs := getTestFS()
	err := wfs.Mkdir(ctx, "")
	require.Nil(t, err)

	// file
	p := "testfile.txt"
	err = wfs.ImportFile(ctx, nil, p)
	require.Nil(t, err)
	o, err := wfs.Lookup(ctx, p)
	require.Nil(t, err)
	require.NotNil(t, o)

	err = wfs.DeleteAt(ctx, p, 0)
	require.Nil(t, err)
	_, err = wfs.Lookup(ctx, p)
	require.Equal(t, ErrNotExist, err)

	// directory
	p = "testdir"
	err = wfs.Mkdir(ctx, p)
	require.Nil(t, err)
	o, err = wfs.Lookup(ctx, p)
	require.Nil(t, err)
	require.NotNil(t, o)

	err = wfs.DeleteAt(ctx, p, 0)
	require.Nil(t, err)
	_, err = wfs.Lookup(ctx, p)
	require.Equal(t, ErrNotExist, err)

	// volume
	p = "vol1"
	_, err = wfs.NewVolume(ctx, p)
	require.Nil(t, err)
	o, err = wfs.Lookup(ctx, p)
	require.Nil(t, err)
	require.NotNil(t, o)

	err = wfs.DeleteAt(ctx, p, 0)
	require.Nil(t, err)
	_, err = wfs.Lookup(ctx, p)
	require.Equal(t, ErrNotExist, err)
}
