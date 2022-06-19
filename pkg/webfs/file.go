package webfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"time"

	"github.com/brendoncarroll/go-state/posixfs"
)

var (
	_ io.Reader   = &File{}
	_ io.ReaderAt = &File{}
)

type File struct {
	vol  *volumeMount
	path string

	ctx    context.Context
	offset int64
}

func newFile(vol *volumeMount, path string) *File {
	return &File{
		vol:  vol,
		path: path,
		ctx:  context.Background(),
	}
}

func (f *File) Read(p []byte) (int, error) {
	n, err := f.ReadAt(p, f.offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	f.offset += int64(n)
	return n, err
}

func (f *File) ReadAt(buf []byte, offset int64) (int, error) {
	root, err := readRoot(f.ctx, f.vol.vol.Cell)
	if err != nil {
		return 0, err
	}
	if root == nil {
		return 0, iofs.ErrNotExist
	}
	if offset < 0 {
		return 0, fmt.Errorf("invalid offset %d", offset)
	}
	s := f.vol.vol.Store
	return f.vol.gotfs.ReadFileAt(f.ctx, s, s, *root, f.path, uint64(offset), buf)
}

func (f *File) Stat() (iofs.FileInfo, error) {
	return f.vol.Stat(f.ctx, f.path)
}

func (f *File) Sync() error {
	return nil
}

func (f *File) ReadDir(n int) (ret []iofs.DirEntry, _ error) {
	return f.vol.readDir(f.ctx, f.path, n)
}

func (f *File) Close() error {
	return nil
}

type fileInfo struct {
	name    string
	mode    iofs.FileMode
	size    int64
	modTime time.Time
}

func (fi fileInfo) Name() string {
	return fi.name
}

func (fi fileInfo) Size() int64 {
	return fi.size
}

func (fi fileInfo) Mode() iofs.FileMode {
	return fi.mode
}

func (fi fileInfo) IsDir() bool {
	return fi.mode.IsDir()
}

func (fi fileInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi fileInfo) Sys() any {
	return nil
}

var _ iofs.DirEntry = &dirEntry{}

type dirEntry struct {
	name    string
	mode    iofs.FileMode
	getInfo func() (*fileInfo, error)
}

func (de *dirEntry) Name() string {
	return de.name
}

func (de *dirEntry) IsDir() bool {
	return de.mode.IsDir()
}

func (de *dirEntry) Type() iofs.FileMode {
	return de.mode.Type()
}

func (de *dirEntry) Info() (iofs.FileInfo, error) {
	return de.getInfo()
}

func convertError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, posixfs.ErrNotExist):
		return iofs.ErrNotExist
	default:
		return err
	}
}
