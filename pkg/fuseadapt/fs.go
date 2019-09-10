package fuseadapt

import (
	"context"
	"errors"

	fusefs "bazil.org/fuse/fs"
	"github.com/brendoncarroll/webfs/pkg/webfs"
)

var _ fusefs.FS = &FS{}

type FS struct {
	wfs *webfs.WebFS
}

func NewFS(wfs *webfs.WebFS) *FS {
	return &FS{wfs: wfs}
}

func (fs *FS) Root() (fusefs.Node, error) {
	ctx := context.TODO()
	o, err := fs.wfs.Lookup(ctx, "")
	if err != nil {
		return nil, err
	}
	d, ok := o.(*webfs.Dir)
	if !ok {
		return nil, errors.New("fuse cannot have none dir object as root")
	}
	return newDir(d), nil
}
