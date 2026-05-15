package webfsfuse

import (
	"context"
	"sort"
	"testing"
	"time"

	"blobcache.io/blobcache/src/bclocal"
	"blobcache.io/blobcache/src/blobcache"
	"blobcache.io/blobcache/src/blobcache/blobcachetests"
	"github.com/brendoncarroll/webfs/src/webfs"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/posixtest"
	"github.com/stretchr/testify/require"
	"go.inet256.org/inet256/src/inet256"
)

func TestPOSIX(t *testing.T) {
	blacklist := map[string]string{
		// This is the only okay skip, we run Blobcache in-process which opens files
		"FdLeak": "counts backend service descriptors that are unrelated to FUSE file-handle leaks",
		// TODO: these changes require a sesison handles table in webfs
		"FstatDeleted": "deleted-open-file inode semantics are not implemented yet",
		"NlinkZero":    "nlink behavior for overwritten open files is not implemented yet",

		"OpenSymlinkRace": "symlink race-hardening semantics are not implemented yet",
	}
	parallel := map[string]struct{}{
		"FcntlFlockLocksFile": {},
		"FcntlFlockSetLk":     {},
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
			if _, ok := parallel[name]; ok {
				t.Parallel()
			}
			mnt, unmount := mountTestFS(t)
			defer unmount()
			fn(t, mnt)
		})
	}
}

func mountTestFS(t *testing.T) (string, func()) {
	t.Helper()

	sys, pki, cfg := setupVolume(t)
	fusefs := New(sys, pki, cfg)
	root := fusefs.Root()
	mountPoint := t.TempDir()

	srv, err := fs.Mount(mountPoint, root, &fs.Options{MountOptions: fuse.MountOptions{EnableLocks: true}})
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

func setupVolume(t testing.TB) (*webfs.System, inet256.PKI, webfs.VolumeConfig) {
	t.Helper()
	ctx := context.Background()
	pki := webfs.DefaultPKI()
	bsvc := bclocal.NewTestService(t)
	sys := webfs.NewSystem(bsvc, pki)

	volh, err := bsvc.CreateVolume(ctx, nil, blobcache.VolumeSpec{
		Local: &blobcache.VolumeBackend_Local{
			HashAlgo: blobcache.HashAlgo_BLAKE2b_256,
			MaxSize:  1 << 21,
		},
	})
	require.NoError(t, err)
	vcfg := sys.GenerateConfig(blobcache.FQOID{Node: blobcachetests.Endpoint(t, bsvc).Node, OID: volh.OID})
	require.NoError(t, sys.Initialize(ctx, *volh, vcfg))
	return sys, pki, vcfg
}
