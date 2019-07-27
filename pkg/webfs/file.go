package webfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"

	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/wrds"
	"golang.org/x/crypto/sha3"
)

type FileMutator func(cur models.File) (*models.File, error)

type File struct {
	m models.File

	baseObject
}

func newFile(ctx context.Context, parent Object, name string) (*File, error) {
	f := &File{
		m: models.File{Tree: wrds.NewTree()},
		baseObject: baseObject{
			parent:       parent,
			nameInParent: name,
			store:        parent.getStore(),
		},
	}
	if err := f.SetData(ctx, nil); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *File) SetData(ctx context.Context, r io.Reader) error {
	if r == nil {
		r = &bytes.Buffer{}
	}
	m := &models.File{Tree: wrds.NewTree()}
	h := sha3.New256()
	buf := make([]byte, f.store.MaxBlobSize())
	if len(buf) == 0 {
		panic("max blob size 0")
	}
	for {
		n, err := r.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		data := buf[:n]

		n, err = h.Write(data)
		if err != nil {
			return err
		}

		ref, err := f.store.Post(ctx, data)
		if err != nil {
			return err
		}

		key := [8]byte{}
		binary.BigEndian.PutUint64(key[:], m.Size)
		m.Tree, err = m.Tree.Put(ctx, f.store, key[:], *ref)
		if err != nil {
			return err
		}

		m.Size += uint64(n)
	}
	m.Checksum = h.Sum(m.Checksum)

	// This is a total replace not a modification of existing content.
	return f.apply(ctx, func(cur models.File) (*models.File, error) {
		return m, nil
	})
}

func (f *File) Lookup(ctx context.Context, p Path) (Object, error) {
	if len(p) == 0 {
		return f, nil
	}
	return nil, errors.New("cannot lookup in file")
}

func (f *File) Walk(ctx context.Context, fn func(Object) bool) (bool, error) {
	cont := fn(f)
	return cont, nil
}

func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	ctx := context.TODO()

	offset := uint64(off)
	for n < len(p) {
		ent, err := f.m.Tree.MaxLteq(ctx, f.store, offset2Key(offset))
		if err != nil {
			return 0, err
		}

		if ent == nil {
			break // done, no entry for this offset, empty file
		}
		o := key2Offset(ent.Key)
		if offset < o {
			return 0, errors.New("got wrong entry from tree")
		}
		relo := offset - o
		data, err := f.store.Get(ctx, ent.Ref)
		if err != nil {
			return n, err
		}
		if int(relo) >= len(data) {
			break // there is no more data, we must have read it all
		}
		n2 := copy(p, data[relo:])
		n += n2
		offset += uint64(n2)
	}
	return n, io.EOF
}

// Split attempts to make the file smaller.
// func (f *File) split(ctx context.Context, store ReadWriteOnce) error {
// 	newTree, err := f.m.Tree.Split(ctx, store)
// 	if err != nil {
// 		return err
// 	}
// 	m := &File{Tree: newTree}
// 	return nil
// 	return &File{Tree: newTree}, nil
// }

func (f *File) Size() uint64 {
	return f.m.Size
}

func (f File) String() string {
	return "Object{File}"
}

func (f *File) RefIter(ctx context.Context, fn func(Ref) bool) (bool, error) {
	return refIterTree(ctx, f.store, f.m.Tree, fn)
}

func (f *File) apply(ctx context.Context, fn FileMutator) error {
	var (
		newFile *models.File
		err     error
	)

	switch x := f.parent.(type) {
	case *Dir:
		err = x.put(ctx, f.nameInParent, func(cur *models.Object) (*models.Object, error) {
			curFile := f.m
			if cur.File != nil {
				curFile = *cur.File
			}

			newFile, err = fn(curFile)
			if err != nil {
				return nil, err
			}
			return &models.Object{File: newFile}, nil
		})
	default:
		panic("invalid parent of file")
	}

	if err != nil {
		return err
	}

	f.m = *newFile
	return nil
}

func (f *File) getStore() ReadWriteOnce {
	return f.store
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
