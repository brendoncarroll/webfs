// Package wfscmd implements the WebFS command line tool
package wfscmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"

	bcclient "blobcache.io/blobcache/client/go"
	"blobcache.io/blobcache/src/blobcache"
	"blobcache.io/blobcache/src/schema/bcns"
	"github.com/brendoncarroll/webfs/src/webfs"
	"github.com/brendoncarroll/webfs/src/webfsfuse"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.brendoncarroll.net/star"
	"golang.org/x/sync/errgroup"
)

func Main() {
	bgCtx := context.Background()
	star.Main(rootCmd, star.MainBackground(bgCtx))
}

var rootCmd = star.NewDir(star.Metadata{
	Short: "webfs is a filesystem built on a web of Blobcache Volumes",
}, map[string]star.Command{
	"mount": mountCmd,
	"init":  initCmd,
	"mkcfg": mkcfgCmd,
	"exec":  execCmd,
})

var mkcfgCmd = star.Command{
	Metadata: star.Metadata{Short: "generate new config and write it to stdout"},
	Flags:    map[string]star.Flag{},
	Pos:      []star.Positional{volExprParam},
	F: func(c star.Context) error {
		ctx := c.Context
		bc := newService()
		pki := webfs.DefaultPKI()
		sys := webfs.NewSystem(bc, pki)

		ep, err := bc.Endpoint(ctx)
		if err != nil {
			return err
		}
		vexpr := volExprParam.Load(c)
		volh, err := vexpr.Open(ctx, bc)
		if err != nil {
			return err
		}
		vcfg := sys.GenerateConfig(blobcache.FQOID{Node: ep.Node, OID: volh.OID})

		data, err := json.MarshalIndent(vcfg, "", "  ")
		if err != nil {
			return err
		}
		_, err = c.StdOut.Write(data)
		return err
	},
}

var initCmd = star.Command{
	Metadata: star.Metadata{Short: "initialize webfs filesystem"},
	Flags: map[string]star.Flag{
		"root": rootConfigParam,
	},
	F: func(c star.Context) error {
		ctx := c.Context
		bc := newService()
		cfg := rootConfigParam.Load(c)
		pki := webfs.DefaultPKI()
		sys := webfs.NewSystem(bc, pki)

		c.Printf("opening volume %v...\n", cfg.VolumeID)
		volh, err := bc.OpenFiat(ctx, cfg.VolumeID, blobcache.Action_ALL)
		if err != nil {
			return err
		}
		c.Printf("opened volume %v\n", cfg.VolumeID)
		if err := sys.Initialize(c, *volh, cfg); err != nil {
			return err
		}
		c.Printf("webfs filesystem successfully initialized in volume %v\n", volh.OID)
		return nil
	},
}

var mountCmd = star.Command{
	Metadata: star.Metadata{Short: "mount a webfs filesystem"},
	Flags: map[string]star.Flag{
		"root": rootConfigParam,
	},
	Pos: []star.Positional{mountDirParam},
	F: func(c star.Context) error {
		ctx, cf := signal.NotifyContext(c.Context, os.Interrupt)
		defer cf()
		bc := newService()
		cfg := rootConfigParam.Load(c)
		pki := webfs.DefaultPKI()
		sys := webfs.NewSystem(bc, pki)
		fsys := webfsfuse.New(sys, pki, cfg)
		node := fsys.Root()

		dirp := mountDirParam.Load(c)
		if dirp == "" {
			return fmt.Errorf("mount dir must not be empty")
		}
		_, err := os.Stat(dirp)
		exists := err == nil
		if !exists {
			if err := os.MkdirAll(dirp, 0o755); err != nil {
				return err
			}
			defer func() {
				os.Remove(dirp)
			}()
		}

		srv, err := fs.Mount(dirp, node, nil)
		if err != nil {
			return err
		}
		defer srv.Unmount()
		go func() {
			log.Printf("unmounting...")
			<-ctx.Done()
			srv.Unmount()
		}()
		log.Printf("mounted filesystem at %s", dirp)
		srv.Wait()
		return nil
	},
}

var execCmd = star.Command{
	Metadata: star.Metadata{
		Short: "run a command with the volume as its PWD, then exit",
	},
	Flags: map[string]star.Flag{
		"root": rootConfigParam,
	},
	Pos: []star.Positional{execPathParam},
	// TODO: need -w flag to allow writing to the volume.
	F: func(c star.Context) error {
		ctx := c.Context
		bc := newService()
		pki := webfs.DefaultPKI()
		sys := webfs.NewSystem(bc, pki)
		dir, err := os.MkdirTemp("", "webfs-")
		if err != nil {
			return err
		}
		cfg := rootConfigParam.Load(c)
		fsys := webfsfuse.New(sys, pki, cfg)
		fusesrv, err := fs.Mount(dir, fsys.Root(), &fs.Options{
			MountOptions: fuse.MountOptions{
				EnableLocks: true,
			},
		})
		if err != nil {
			return err
		}
		var eg errgroup.Group
		eg.Go(func() error {
			fusesrv.Wait()
			return nil
		})
		execPath := execPathParam.Load(c)

		cmd := exec.CommandContext(ctx, execPath)
		cmd.Dir = dir
		cmd.Stdout = c.StdOut
		cmd.Stdin = c.StdIn
		if err := cmd.Run(); err != nil {
			return err
		}
		return fusesrv.Unmount()
	},
}

var mountDirParam = &star.Required[string]{
	PosName:  "mount",
	ShortDoc: "path to the directory where the filesystem will be mounted",
	Parse:    star.ParseString,
}

var rootConfigParam = &star.Required[webfs.VolumeConfig]{
	ShortDoc: "path to root volume configuration",
	Parse: func(x string) (cfg webfs.VolumeConfig, _ error) {
		data, err := os.ReadFile(x)
		if err != nil {
			return webfs.VolumeConfig{}, err
		}
		return cfg, json.Unmarshal(data, &cfg)
	},
}

var volExprParam = &star.Required[bcns.ObjectExpr]{
	PosName:  "volume",
	ShortDoc: "blobcache object expression",
	Parse:    bcns.ParseObjectish,
}

var execPathParam = &star.Required[string]{
	PosName:  "exec-path",
	ShortDoc: "path to executable",
	Parse:    star.ParseString,
}

func newService() blobcache.Service {
	return bcclient.NewClientFromEnv()
}
