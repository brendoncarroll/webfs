package webfs

import (
	"errors"
	"io"
)

func (f *File) GetHandle() *FileHandle {
	return newFileHandle(f)
}

func newFileHandle(f *File) *FileHandle {
	return &FileHandle{
		file: f,
	}
}

type FileHandle struct {
	offset int64
	file   *File
}

func (fh *FileHandle) Seek(offset int64, whence int) (ret int64, err error) {
	switch whence {
	case io.SeekStart:
		fh.offset = offset
	case io.SeekCurrent:
		fh.offset = fh.offset + offset
	case io.SeekEnd:
		o := int64(fh.file.Size()) - offset
		if o < 0 {
			return 0, errors.New("negative offset")
		}
		fh.offset = int64(o)
	default:
		panic("invalid value for whence")
	}
	return fh.offset, nil
}

func (fh *FileHandle) Write(p []byte) (n int, err error) {
	n, err = fh.file.WriteAt(p, fh.offset)
	fh.offset += int64(n)
	return n, err
}

func (fh *FileHandle) Read(p []byte) (n int, err error) {
	n, err = fh.file.ReadAt(p, fh.offset)
	fh.offset += int64(n)
	return n, err
}

func (fh *FileHandle) Close() error {
	return nil
}
