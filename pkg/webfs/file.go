package webfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"

	"github.com/brendoncarroll/webfs/pkg/webref"
	"github.com/brendoncarroll/webfs/pkg/wrds"
	"golang.org/x/crypto/sha3"
)

type File struct {
	Checksum []byte `json:"checksum"`
	Size     uint64 `json:"size"`

	Tree *wrds.Tree `json:"tree"`
}

func NewFile(ctx context.Context, s ReadWriteOnce, r io.Reader) (*File, error) {
	if r == nil {
		r = &bytes.Buffer{}
	}
	f := &File{Tree: wrds.NewTree()}
	h := sha3.New256()

	buf := make([]byte, s.MaxBlobSize())
	if len(buf) == 0 {
		panic("max blob size 0")
	}
	for {
		n, err := r.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		data := buf[:n]

		n, err = h.Write(data)
		if err != nil {
			return nil, err
		}

		ref, err := s.Post(ctx, data)
		if err != nil {
			return nil, err
		}

		key := [8]byte{}
		binary.BigEndian.PutUint64(key[:], f.Size)
		f.Tree, err = f.Tree.Put(ctx, s, key[:], *ref)
		if err != nil {
			return nil, err
		}

		f.Size += uint64(n)
	}
	f.Checksum = h.Sum(f.Checksum)

	return f, nil
}

// Split attempts to make the file smaller.
func (f *File) Split(ctx context.Context, store ReadWriteOnce) (*File, error) {
	newTree, err := f.Tree.Split(ctx, store)
	if err != nil {
		return nil, err
	}
	return &File{Tree: newTree}, nil
}

func (f *File) RefIter(ctx context.Context, store Read, fn func(Ref) bool) (bool, error) {
	return refIterTree(ctx, store, f.Tree, fn)
}

func (f *File) GetHandle(store webref.Read) *FileHandle {
	return newFileHandle(store, f)
}

func newFileHandle(store webref.Read, f *File) *FileHandle {
	return &FileHandle{
		file:  f,
		store: store,
	}
}

type FileHandle struct {
	store  webref.Read
	offset uint64
	file   *File

	ti *wrds.TreeIter
}

func (fh *FileHandle) Seek(offset int64, whence int) (ret int64, err error) {
	switch whence {
	case io.SeekStart:
		fh.offset = uint64(offset)
	case io.SeekCurrent:
		fh.offset = uint64(int64(fh.offset) + offset)
	case io.SeekEnd:
		o := int64(fh.file.Size) - offset
		if o < 0 {
			return 0, errors.New("negative offset")
		}
		fh.offset = uint64(o)
	default:
		panic("invalid value for whence")
	}
	return int64(fh.offset), nil
}

func (fh *FileHandle) Read(p []byte) (n int, err error) {
	ctx := context.TODO()

	for n < len(p) {
		ent, err := fh.file.Tree.MaxLteq(ctx, fh.store, offset2Key(fh.offset))
		if err != nil {
			return 0, err
		}

		if ent == nil {
			break // done, no entry for this offset, empty file
		}
		o := key2Offset(ent.Key)
		if fh.offset < o {
			return 0, errors.New("got wrong entry from tree")
		}
		relo := fh.offset - o
		data, err := fh.store.Get(ctx, ent.Ref)
		if err != nil {
			return n, err
		}
		if int(relo) >= len(data) {
			break // there is no more data, we must have read it all
		}
		n2 := copy(p, data[relo:])
		n += n2
		fh.offset += uint64(n2)
	}
	return n, io.EOF
}

func (fr *FileHandle) Close() error {
	return nil
}

func offset2Key(x uint64) []byte {
	y := [8]byte{}
	binary.BigEndian.PutUint64(y[:], x)
	return y[:]
}

func key2Offset(x []byte) uint64 {
	return binary.BigEndian.Uint64(x)
}

func refIterTree(ctx context.Context, store Read, t *wrds.Tree, f func(Ref) bool) (bool, error) {
	iter, err := t.Iterate(ctx, store, nil)
	if err != nil {
		return false, err
	}

	cont := true
	for cont {
		entry, err := iter.Next(ctx)
		if err != nil {
			return false, err
		}
		if entry == nil {
			return false, nil
		}
		cont = f(entry.Ref)
	}
	return cont, nil
}
