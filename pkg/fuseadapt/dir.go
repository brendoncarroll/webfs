package fuseadapt

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/brendoncarroll/webfs/pkg/webfs"
)

var _ interface {
	fusefs.Node
	fusefs.NodeRenamer
	fusefs.NodeMkdirer
	fusefs.NodeLinker
	fusefs.NodeRemover
} = &Dir{}

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
		return nil, fuseErr(err)
	}
	switch x := o.(type) {
	case *webfs.Dir:
		return newDir(x), nil
	case *webfs.File:
		return newFile(x), nil
	default:
		err = errors.New("cannot create fuse node from webfs object")
		log.Println(err)
		return nil, err
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

func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fusefs.Node, error) {
	d2, err := webfs.NewDir(ctx, d.d, req.Name)
	if err != nil {
		return nil, err
	}
	return newDir(d2), nil
}

func (d *Dir) Link(ctx context.Context, req *fuse.LinkRequest, old fusefs.Node) (fusefs.Node, error) {
	o := old.(getObject).getObject()
	fs := d.d.FS()
	dst := append(d.d.Path(), req.NewName).String()
	oCopy, err := fs.Copy(ctx, o, dst)
	if err != nil {
		return nil, err
	}
	return wrap(oCopy), nil
}

func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fusefs.Node) error {
	o, err := d.d.Lookup(ctx, webfs.Path{req.OldName})
	if err != nil {
		return fuseErr(err)
	}
	dst := newDir.(*Dir).d.Path()
	dst = append(dst, req.NewName)

	fs := d.d.FS()
	return fs.Move(ctx, o, dst.String())
}

func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	return d.d.Delete(ctx, req.Name)
}

func (d *Dir) getObject() webfs.Object {
	return d.d
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

func fuseErr(err error) error {
	switch err {
	case os.ErrNotExist:
		return fuse.ENOENT
	case os.ErrExist:
		return fuse.EEXIST
	default:
		return err
	}
}
