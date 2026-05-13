package webfs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"blobcache.io/blobcache/src/bclocal"
	"blobcache.io/blobcache/src/blobcache"
	"github.com/stretchr/testify/require"
	"go.brendoncarroll.net/tai64"
)

func TestFile(t *testing.T) {
	ctx := context.Background()
	sys, vcfg := setupVol(t)
	var fileIno INode
	contents := "hello world"
	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		ino, err := tx.CreateFileAt(ctx, INode{}, "a", 0o755, FileParams{
			Now:       tai64.Now(),
			BlockSize: 4096,
		})
		require.NoError(t, err)
		if err := tx.WriteAt(ctx, ino, 0, []byte(contents)); err != nil {
			return err
		}
		fileIno = ino
		return nil
	}))
	require.NoError(t, sys.View(ctx, vcfg, func(tx *Tx) error {
		buf := make([]byte, 100)
		n, err := tx.ReadAt(ctx, fileIno, 0, buf)
		actual := string(buf[:n])
		require.Equal(t, contents, actual)
		return err
	}))
}

func TestDir(t *testing.T) {
	ctx := context.Background()
	sys, vcfg := setupVol(t)
	contents := []byte("hello world")
	var fileIno INode

	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		ino, err := tx.CreateFileAt(ctx, rootINode(), "a", 0o644, FileParams{
			Now:       tai64.Now(),
			BlockSize: 4096,
		})
		if err != nil {
			return err
		}
		fileIno = ino
		if err := tx.WriteAt(ctx, fileIno, 0, contents); err != nil {
			return err
		}
		if err := tx.Link(ctx, rootINode(), "b", 0o644, fileIno); err != nil {
			return err
		}

		a, err := tx.GetChild(ctx, rootINode(), "a")
		if err != nil {
			return err
		}
		b, err := tx.GetChild(ctx, rootINode(), "b")
		if err != nil {
			return err
		}
		require.Equal(t, a.Target, b.Target)

		if err := tx.Unlink(ctx, rootINode(), "a"); err != nil {
			return err
		}
		if _, err := tx.GetChild(ctx, rootINode(), "a"); err == nil || !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("expected a to be missing after unlink, err=%v", err)
		}

		buf := make([]byte, len(contents))
		n, err := tx.ReadAt(ctx, fileIno, 0, buf)
		if err != nil {
			return err
		}
		require.Equal(t, len(contents), n)
		require.Equal(t, string(contents), string(buf[:n]))

		if err := tx.Unlink(ctx, rootINode(), "b"); err != nil {
			return err
		}
		if _, err := tx.GetChild(ctx, rootINode(), "b"); err == nil || !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("expected b to be missing after unlink, err=%v", err)
		}
		if _, err := tx.ReadAt(ctx, fileIno, 0, make([]byte, 1)); err == nil {
			return fmt.Errorf("expected file inode to be deleted after final unlink")
		}
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
