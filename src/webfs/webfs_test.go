package webfs

import (
	"context"
	"testing"

	"blobcache.io/blobcache/src/bclocal"
	"blobcache.io/blobcache/src/blobcache"
	"github.com/stretchr/testify/require"
	"go.brendoncarroll.net/tai64"
)

func TestFile(t *testing.T) {
	ctx := context.Background()
	sys, vcfg := setupVol(t)
	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		ino, err := tx.CreateFileAt(ctx, rootINode(), "a", 0o755, FileParams{
			Now:       tai64.Now(),
			BlockSize: 4096,
		})
		require.NoError(t, err)
		if err := tx.WriteAt(ctx, ino, 0, []byte("hello world")); err != nil {
			return err
		}
		return nil
	}))
	require.NoError(t, sys.View(ctx, vcfg, func(tx *Tx) error {
		return nil
	}))
}

func setupVol(t testing.TB) (*System, VolumeConfig) {
	ctx := context.Background()
	bc := bclocal.NewTestService(t)
	volh, err := bc.CreateVolume(ctx, nil, blobcache.VolumeSpec{
		Local: &blobcache.VolumeBackend_Local{
			HashAlgo: blobcache.HashAlgo_BLAKE2b_256,
			MaxSize:  1 << 21,
		},
	})
	require.NoError(t, err)
	sys := NewSystem(bc)
	vcfg, err := sys.Initialize(ctx, *volh)
	require.NoError(t, err)
	return sys, vcfg
}
