package fuseadapt

import (
	"context"
	"errors"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/brendoncarroll/webfs/pkg/webfs"
)

type Dir struct {
	d *webfs.Dir
}

func newDir(d *webfs.Dir) *Dir {
	return &Dir{d: d}
}

func (d *Dir) Attr(ctx context.Context, attr *fuse.Attr) error {
	*attr = toAttr(d.d.FileInfo())
	return nil
}

func (d *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fusefs.Node, error) {
	o, err := d.d.Lookup(ctx, webfs.ParsePath(req.Name))
	if err != nil {
		return nil, err
	}
	switch x := o.(type) {
	case *webfs.Dir:
		return newDir(x), nil
	case *webfs.File:
		return newFile(x), nil
	default:
		return nil, errors.New("cannot create fuse node from webfs object")
	}
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	ents, err := d.d.Entries(ctx)
	if err != nil {
		return nil, err
	}
	fuseEnts := make([]fuse.Dirent, len(ents))
	for i := range ents {
		fuseEnts[i] = toDirent(ents[i])
	}
	return fuseEnts, nil
}

func toDirent(x webfs.DirEntry) fuse.Dirent {
	var ty fuse.DirentType = fuse.DT_Unknown
	switch x.Object.(type) {
	case *webfs.Dir:
		ty = fuse.DT_Dir
	case *webfs.File:
		ty = fuse.DT_File
	}

	return fuse.Dirent{
		Name: x.Name,
		Type: ty,
	}
}

func toAttr(finfo webfs.FileInfo) fuse.Attr {
	return fuse.Attr{
		Valid: time.Minute,

		Atime: finfo.AccessedAt,
		Ctime: finfo.CreatedAt,
		Mtime: finfo.ModifiedAt,

		//BlockSize: 512,
		//Blocks:    finfo.Size / 512,
		Size: finfo.Size,
		Mode: finfo.Mode,
	}
}
