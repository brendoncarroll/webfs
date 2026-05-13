package webfscmd

import (
	"context"

	bcclient "blobcache.io/blobcache/client/go"
	"blobcache.io/blobcache/src/blobcache"
	"github.com/brendoncarroll/webfs/src/webfs"
	"github.com/brendoncarroll/webfs/src/webfsfuse"
	"github.com/hanwen/go-fuse/v2/fs"
	"go.brendoncarroll.net/star"
)

func Main() {
	bgCtx := context.Background()
	star.Main(rootCmd, star.MainBackground(bgCtx))
}

var rootCmd = star.NewDir(star.Metadata{Short: "webfs is a filesystem built on a web of Blobcache Volumes"},
	map[string]star.Command{
		"mount": mountCmd,
	})

var mountCmd = star.Command{
	Metadata: star.Metadata{Short: "mount"},
	Flags: map[string]star.Flag{
		"root": rootVolParam,
	},
	Pos: []star.Positional{mountDirParam},
	F: func(c star.Context) error {
		bc := newService()
		cfg, err := getVolConfig(c)
		if err != nil {
			return err
		}
		sys := webfs.NewSystem(bc, cfg)
		node := webfsfuse.NewRoot(sys)
		srv, err := fs.Mount(mountDirParam.Load(c), node, nil)
		if err != nil {
			return err
		}
		defer srv.Unmount()
		srv.Serve()
		return nil
	},
}

func getVolConfig(c star.Context) (webfs.VolumeConfig, error) {
	// TODO:
	return webfs.VolumeConfig{}, nil
}

var mountDirParam = &star.Required[string]{
	PosName:  "mount",
	ShortDoc: "path to the directory where the filesystem will be mounted",
	Parse:    star.ParseString,
}

var rootVolParam = &star.Required[blobcache.OID]{
	ShortDoc: "root volume OID",
	Parse:    blobcache.ParseOID,
}

func newService() blobcache.Service {
	return bcclient.NewClientFromEnv()
}
