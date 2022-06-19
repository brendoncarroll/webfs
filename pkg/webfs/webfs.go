package webfs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	iofs "io/fs"
	"path"
	"strings"
	"time"

	"github.com/brendoncarroll/go-state/cadata"
	"github.com/brendoncarroll/go-state/cells"
	"github.com/brendoncarroll/go-state/posixfs"
	"github.com/gotvc/got/pkg/gdat"
	"github.com/gotvc/got/pkg/gotfs"
	"github.com/sirupsen/logrus"
)

const (
	MaxBlobSize = gotfs.DefaultMaxBlobSize
)

func Hash(x []byte) cadata.ID {
	return gdat.Hash(x)
}

type Volume struct {
	Cell  cells.Cell
	Store cadata.Store
	Salt  []byte
}

// FS is an instance of a WebFS filesystem
type FS struct {
	config *fsConfig
	fs     posixfs.FS
	log    logrus.FieldLogger

	root *volumeMount
}

func New(vspec VolumeSpec, opts ...Option) (*FS, error) {
	config := defaultConfig()
	for _, opt := range opts {
		opt(&config)
	}
	fs := &FS{
		config: &config,
		fs:     config.pfs,
		log:    config.log,
	}
	root, err := fs.getVolumeMount(context.Background(), nil, "", &vspec)
	if err != nil {
		return nil, err
	}
	fs.root = root
	return fs, nil
}

func (fs *FS) Open(ctx context.Context, p string) (*File, error) {
	res, err := fs.resolve(ctx, fs.root, p)
	if err != nil {
		return nil, err
	}
	fs.log.Infof("open %q", p)
	return res.VM.Open(res.Path)
}

func (fs *FS) PutFile(ctx context.Context, p string, r io.Reader) error {
	res, err := fs.resolve(ctx, fs.root, p)
	if err != nil {
		return err
	}
	return res.VM.PutFile(ctx, res.Path, r)
}

func (fs *FS) Mkdir(ctx context.Context, p string) error {
	res, err := fs.resolve(ctx, fs.root, p)
	if err != nil {
		return err
	}
	return res.VM.Mkdir(ctx, res.Path)
}

func (fs *FS) Remove(ctx context.Context, p string) error {
	res, err := fs.resolve(ctx, fs.root, p)
	if err != nil {
		return err
	}
	return res.VM.Rm(ctx, p)
}

func (fs *FS) Cat(ctx context.Context, p string, w io.Writer) error {
	f, err := fs.Open(ctx, p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

func (fs *FS) Ls(ctx context.Context, p string, fn func(iofs.DirEntry) error) error {
	f, err := fs.Open(ctx, p)
	if err != nil {
		return err
	}
	defer f.Close()
	dirEnts, err := f.ReadDir(0)
	if err != nil {
		return err
	}
	for _, dirEnt := range dirEnts {
		if err := fn(dirEnt); err != nil {
			return err
		}
	}
	return nil
}

func (fs *FS) getVolumeMount(ctx context.Context, parent *volumeMount, p string, spec *VolumeSpec) (*volumeMount, error) {
	vol, err := fs.makeVolume(*spec)
	if err != nil {
		return nil, err
	}
	var seed [32]byte
	copy(seed[:], spec.Salt)
	return &volumeMount{
		parent: parent,
		path:   p,
		vol:    *vol,
		gotfs:  gotfs.NewOperator(gotfs.WithSeed(&seed), gotfs.WithContentCacheSize(10), gotfs.WithMetaCacheSize(128)),
	}, nil
}

type resolveRes struct {
	VM   *volumeMount
	Path string
}

func (fs *FS) resolve(ctx context.Context, vm *volumeMount, p string) (*resolveRes, error) {
	p = cleanPath(p)
	root, err := readRoot(ctx, vm.vol.Cell)
	if err != nil {
		return nil, err
	}
	if root != nil {
		for _, configPath := range potConfigPaths(p) {
			vs, err := loadWebFSConfig(ctx, &vm.gotfs, vm.vol.Store, *root, configPath)
			if err != nil {
				return nil, err
			}
			if vs == nil {
				continue
			}
			mountPath := path.Dir(configPath)
			vm2, err := fs.getVolumeMount(ctx, vm, mountPath, vs)
			if err != nil {
				return nil, err
			}
			p2 := cleanPath(p[len(mountPath):])
			return fs.resolve(ctx, vm2, p2)
		}
	}
	return &resolveRes{
		VM:   vm,
		Path: p,
	}, nil
}

type volumeMount struct {
	parent *volumeMount
	path   string

	vol   Volume
	gotfs gotfs.Operator
}

func (v *volumeMount) Open(p string) (*File, error) {
	return newFile(v, p), nil
}

func (v *volumeMount) PutFile(ctx context.Context, p string, r io.Reader) error {
	p = cleanPath(p)
	ms, ds := v.vol.Store, v.vol.Store
	return modifyRoot(ctx, v.vol.Cell, func(root *gotfs.Root) (*gotfs.Root, error) {
		var err error
		if root == nil {
			root, err = v.gotfs.NewEmpty(ctx, ms)
			if err != nil {
				return nil, err
			}
		}
		root, err = v.gotfs.RemoveAll(ctx, ms, *root, p)
		if err != nil {
			return nil, err
		}
		if p != "" {
			root, err = v.gotfs.MkdirAll(ctx, ms, *root, parentOf(p))
			if err != nil {
				return nil, err
			}
		}
		root, err = v.gotfs.CreateFile(ctx, ms, ds, *root, p, r)
		if err != nil {
			return nil, err
		}
		return root, nil
	})
}

func (v *volumeMount) Rm(ctx context.Context, p string) error {
	ms := v.vol.Store
	return modifyRoot(ctx, v.vol.Cell, func(root *gotfs.Root) (*gotfs.Root, error) {
		var err error
		if root == nil {
			return nil, nil
		}
		root, err = v.gotfs.RemoveAll(ctx, ms, *root, p)
		if err != nil {
			return nil, err
		}
		return root, nil
	})
}

func (v *volumeMount) Stat(ctx context.Context, p string) (iofs.FileInfo, error) {
	p = cleanPath(p)
	root, err := readRoot(ctx, v.vol.Cell)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return nil, iofs.ErrNotExist
	}
	return v.stat(ctx, *root, p)
}

func (v *volumeMount) Mkdir(ctx context.Context, p string) error {
	p = cleanPath(p)
	ms := v.vol.Store
	return modifyRoot(ctx, v.vol.Cell, func(root *gotfs.Root) (*gotfs.Root, error) {
		var err error
		if root == nil {
			root, err = v.gotfs.NewEmpty(ctx, ms)
			if err != nil {
				return nil, err
			}
		}
		return v.gotfs.MkdirAll(ctx, ms, *root, p)
	})
}

func (v *volumeMount) readDir(ctx context.Context, p string, n int) (ret []iofs.DirEntry, _ error) {
	root, err := readRoot(ctx, v.vol.Cell)
	if err != nil {
		return nil, err
	}
	if root == nil {
		if p != "" {
			return nil, iofs.ErrNotExist
		}
		return nil, nil
	}
	stopIter := errors.New("stop iteration")
	if err := v.gotfs.ReadDir(ctx, v.vol.Store, *root, p, func(e gotfs.DirEnt) error {
		if n > 0 && len(ret) >= n {
			return stopIter
		}
		ret = append(ret, &dirEntry{
			name: e.Name,
			mode: e.Mode,
			getInfo: func() (*fileInfo, error) {
				return v.stat(ctx, *root, path.Join(v.path, e.Name))
			},
		})
		return nil
	}); err != nil && !errors.Is(err, stopIter) {
		return nil, err
	}
	return ret, nil
}

func (v *volumeMount) stat(ctx context.Context, root gotfs.Root, p string) (*fileInfo, error) {
	ms := v.vol.Store
	info, err := v.gotfs.GetInfo(ctx, ms, root, p)
	if err != nil {
		return nil, convertError(err)
	}
	mode := iofs.FileMode(info.Mode)
	var size int64
	if mode.IsRegular() {
		s, err := v.gotfs.SizeOfFile(ctx, ms, root, p)
		if err != nil {
			return nil, convertError(err)
		}
		size = int64(s)
	}
	return &fileInfo{
		name:    path.Base(p),
		mode:    mode,
		size:    size,
		modTime: time.Now(),
	}, nil
}

func readRoot(ctx context.Context, c cells.Cell) (*gotfs.Root, error) {
	data, err := cells.GetBytes(ctx, c)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var root gotfs.Root
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	return &root, nil
}

func modifyRoot(ctx context.Context, c cells.Cell, fn func(*gotfs.Root) (*gotfs.Root, error)) error {
	return cells.Apply(ctx, c, func(x []byte) ([]byte, error) {
		var xRoot *gotfs.Root
		if len(x) > 0 {
			xRoot = &gotfs.Root{}
			if err := json.Unmarshal(x, xRoot); err != nil {
				return nil, err
			}
		}
		yRoot, err := fn(xRoot)
		if err != nil {
			return nil, err
		}
		if yRoot == nil {
			return nil, nil
		}
		return json.Marshal(yRoot)
	})
}

func cleanPath(x string) string {
	x = strings.Trim(x, "/")
	return x
}

func parentOf(x string) string {
	parts := strings.Split(x, "/")
	if len(parts) == 1 {
		return ""
	}
	return cleanPath(strings.Join(parts[:len(parts)-1], "/"))
}

// potConfigPath returns a list of the potential config paths
func potConfigPaths(p string) (ret []string) {
	p = cleanPath(p)
	parts := strings.Split(p, "/")
	for i, x := range parts {
		if x == "" {
			continue
		}
		configPath := strings.Join(parts[:i+1], "/") + ".webfs"
		ret = append(ret, configPath)
	}
	return ret
}

func loadWebFSConfig(ctx context.Context, fsop *gotfs.Operator, s cadata.Store, root gotfs.Root, p string) (*VolumeSpec, error) {
	const maxConfigSize = 1 << 16
	info, err := fsop.GetInfo(ctx, s, root, p)
	if posixfs.IsErrNotExist(err) {
		return nil, nil
	}
	if !posixfs.FileMode(info.Mode).IsRegular() {
		return nil, nil
	}
	r := fsop.NewReader(ctx, s, s, root, p)
	buf := make([]byte, maxConfigSize)
	n, err := io.ReadFull(r, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	vs, err := ParseVolumeSpec(buf[:n])
	if err != nil {
		return nil, ErrBadConfig{
			Path:  p,
			Data:  buf[:n],
			Inner: err,
		}
	}
	return vs, nil
}
