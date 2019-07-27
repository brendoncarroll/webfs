package webui

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/brendoncarroll/webfs/pkg/webfs"
)

var (
	_ http.File       = &File{}
	_ http.FileSystem = &FileSystem{}
	_ os.FileInfo     = &FileInfo{}
)

func ServeUI(wfs *webfs.WebFS) error {
	fs := &FileSystem{wfs: wfs}
	handler := http.FileServer(fs)
	l, err := net.Listen("tcp", "127.0.0.1:8025")
	if err != nil {
		return err
	}
	u := "http://" + l.Addr().String()
	log.Println("webui on", u)
	return http.Serve(l, handler)
}

type FileSystem struct {
	wfs *webfs.WebFS
}

func (fs *FileSystem) Open(p string) (http.File, error) {
	ctx := context.TODO()
	o, err := fs.wfs.Lookup(ctx, p)
	if err != nil {
		return nil, err
	}
	log.Println("open", o.Path())
	return &File{
		o:    o,
		path: p,
	}, nil
}

type File struct {
	o    webfs.Object
	fh   *webfs.FileHandle
	path string
}

func (f *File) Close() error {
	return f.fh.Close()
}

func (f *File) Read(p []byte) (n int, err error) {
	wf, ok := f.o.(*webfs.File)
	if !ok {
		return 0, errors.New("cannot read non-file")
	}

	if f.fh == nil {
		f.fh = wf.GetHandle()
	}
	return f.fh.Read(p)
}

func (f *File) Seek(off int64, whence int) (int64, error) {
	wf, ok := f.o.(*webfs.File)
	if !ok {
		return 0, errors.New("cannot read non-file")
	}

	if f.fh == nil {
		f.fh = wf.GetHandle()
	}
	return f.fh.Seek(off, whence)
}

func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	ctx := context.TODO()
	wd, ok := f.o.(*webfs.Dir)
	if !ok {
		return nil, errors.New("cannot readdir non-dir")
	}

	entries, err := wd.Entries(ctx)
	if err != nil {
		return nil, err
	}
	finfos := make([]os.FileInfo, len(entries))
	for i, e := range entries {
		_, isDir := e.Object.(*webfs.Dir)
		finfos[i] = &FileInfo{
			name:    e.Name,
			isDir:   isDir,
			size:    int64(wd.Size()),
			modTime: time.Now(),
			sys:     nil,
		}
	}
	return finfos, nil
}

func (f *File) Stat() (os.FileInfo, error) {
	_, isDir := f.o.(*webfs.Dir)
	fi := &FileInfo{
		name:    path.Base(f.path),
		mode:    0644,
		isDir:   isDir,
		size:    int64(f.o.Size()),
		modTime: time.Now(),
		sys:     nil,
	}
	return fi, nil
}

type FileInfo struct {
	name    string
	isDir   bool
	modTime time.Time
	mode    os.FileMode
	size    int64
	sys     interface{}
}

func (fi *FileInfo) IsDir() bool {
	return fi.isDir
}

func (fi *FileInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi *FileInfo) Mode() os.FileMode {
	return fi.mode
}

func (fi *FileInfo) Name() string {
	return fi.name
}

func (fi *FileInfo) Size() int64 {
	return fi.size
}

func (fi *FileInfo) Sys() interface{} {
	return fi.sys
}
