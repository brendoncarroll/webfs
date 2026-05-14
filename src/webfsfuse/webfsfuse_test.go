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
		"FdLeak":                     "counts backend service descriptors that are unrelated to FUSE file-handle leaks",
		"DirSeek":                    "directory seek semantics are not implemented yet",
		"DirectIO":                   "O_DIRECT semantics are not implemented yet",
		"Fallocate":                  "fallocate is not implemented yet",
		"FallocateKeepSize":          "fallocate keep-size is not implemented yet",
		"FcntlFlockLocksFile":        "fcntl file locking is not implemented yet",
		"FcntlFlockSetLk":            "fcntl file locking is not implemented yet",
		"Link":                       "hard-link operations are not implemented yet",
		"LinkUnlinkRename":           "link/rename semantics are not implemented yet",
		"LseekEnxioCheck":            "SEEK_DATA/SEEK_HOLE is not implemented yet",
		"LseekHoleSeeksToEOF":        "SEEK_DATA/SEEK_HOLE is not implemented yet",
		"NlinkZero":                  "nlink reporting semantics are not implemented yet",
		"OpenAt":                     "openat path/rename semantics are not implemented yet",
		"OpenSymlinkRace":            "symlink handling is not implemented yet",
		"RenameOpenDir":              "rename semantics are not implemented yet",
		"RenameOverwriteDestExist":   "rename overwrite semantics are not implemented yet",
		"RenameOverwriteDestNoExist": "rename overwrite semantics are not implemented yet",
		"SetattrSymlink":             "symlink setattr semantics are not implemented yet",
		"SymlinkReadlink":            "symlink operations are not implemented yet",
		"XAttr":                      "xattr operations are not implemented yet",
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
