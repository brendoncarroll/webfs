package webfsim

import (
	"context"
	"io"
	"time"

	webref "github.com/brendoncarroll/webfs/pkg/webref"
)

type FileReader struct {
	pos int64
	f   *File
	s   webref.Getter

	readTimeout time.Duration
}

func NewFileReader(s webref.Getter, f *File) *FileReader {
	return &FileReader{
		pos: 0,
		f:   f,
		s:   s,
	}
}

func (fr *FileReader) Read(p []byte) (n int, err error) {
	var (
		ctx = context.Background()
		cf  context.CancelFunc
	)
	if fr.readTimeout > 0 {
		ctx, cf = context.WithTimeout(context.Background(), fr.readTimeout)
		defer cf()
	}

	n, err = FileReadAt(ctx, fr.s, *fr.f, uint64(fr.pos), p)
	fr.pos += int64(n)
	return n, err
}

func (fr *FileReader) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		fr.pos += off
	case io.SeekStart:
		fr.pos = off
	case io.SeekEnd:
		fr.pos = int64(fr.f.Size) - off
	default:
		panic("bad value for seek")
	}
	return fr.pos, nil
}

func (fr *FileReader) SetReadTimeout(t time.Duration) {
	fr.readTimeout = t
}
