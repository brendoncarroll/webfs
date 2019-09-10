package fuseadapt

import (
	"context"
	"io"

	"bazil.org/fuse"
	"github.com/brendoncarroll/webfs/pkg/webfs"
)

type File struct {
	f *webfs.File
}

func newFile(f *webfs.File) *File {
	return &File{f: f}
}

func (f *File) Attr(ctx context.Context, attr *fuse.Attr) error {
	*attr = toAttr(f.f.FileInfo())
	return nil
}

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	buf := make([]byte, req.Size)
	n, err := f.f.ReadAt(buf, req.Offset)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		return err
	}

	resp.Data = buf[:n]
	return nil
}
