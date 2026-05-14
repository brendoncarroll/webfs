package webfsfuse

import (
	"context"
	"sort"
	"testing"
	"time"

	"blobcache.io/blobcache/src/bclocal"
	"blobcache.io/blobcache/src/blobcache"
	"github.com/brendoncarroll/webfs/src/webfs"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/posixtest"
	"github.com/stretchr/testify/require"
)

func TestPOSIX(t *testing.T) {
	blacklist := map[string]string{
		"FdLeak":              "counts backend service descriptors that are unrelated to FUSE file-handle leaks",
		"FcntlFlockLocksFile": "fcntl file locking is not implemented yet",
		"FcntlFlockSetLk":     "fcntl file locking is not implemented yet",
		"FstatDeleted":        "deleted-open-file inode semantics are not implemented yet",
		"NlinkZero":           "nlink behavior for overwritten open files is not implemented yet",
		"OpenSymlinkRace":     "symlink race-hardening semantics are not implemented yet",
		"RenameOpenDir":       "rename-over-directory semantics are not implemented yet",
		"XAttr":               "xattr operations are not implemented yet",
	}

	names := make([]string, 0, len(posixtest.All))
	for name := range posixtest.All {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if reason, excluded := blacklist[name]; excluded {
			t.Run(name, func(t *testing.T) {
				t.Skip(reason)
			})
			continue
		}

		fn := posixtest.All[name]
		t.Run(name, func(t *testing.T) {
			mnt, unmount := mountTestFS(t)
			defer unmount()
			fn(t, mnt)
		})
	}
}

func mountTestFS(t *testing.T) (string, func()) {
	t.Helper()

	sys, cfg := setupVolume(t)
	root := NewRoot(sys, cfg)
	mountPoint := t.TempDir()

	srv, err := fs.Mount(mountPoint, root, &fs.Options{})
	require.NoError(t, err)
	unmount := func() {
		for i := 0; i < 20; i++ {
			if err := srv.Unmount(); err == nil {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	if err := srv.WaitMount(); err != nil {
		_ = srv.Unmount()
		require.NoError(t, err)
	}
	return mountPoint, unmount
}

func setupVolume(t testing.TB) (*webfs.System, webfs.VolumeConfig) {
	t.Helper()
	ctx := context.Background()
	bsvc := bclocal.NewTestService(t)
	volh, err := bsvc.CreateVolume(ctx, nil, blobcache.VolumeSpec{
		Local: &blobcache.VolumeBackend_Local{
			HashAlgo: blobcache.HashAlgo_BLAKE2b_256,
			MaxSize:  1 << 21,
		},
	})
	require.NoError(t, err)
	sys := webfs.NewSystem(bsvc)
	vcfg, err := sys.Initialize(ctx, *volh)
	require.NoError(t, err)
	return sys, vcfg
}
