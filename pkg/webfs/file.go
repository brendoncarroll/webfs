package webfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"

	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/wrds"
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
		m, err = fileAppend(ctx, f.store, *m, data)
		if err != nil {
			return err
		}
	}

	// This is a total replace not a modification of existing content.
	return f.apply(ctx, func(cur models.File) (*models.File, error) {
		return m, nil
	})
}

func (f *File) Find(ctx context.Context, p Path, objs []Object) ([]Object, error) {
	if len(p) == 0 {
		return []Object{f}, nil
	}
	return nil, errors.New("cannot lookup in file")
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
	return fileReadAt(ctx, f.store, f.m, offset, p)
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
			if cur != nil {
				of, ok := cur.Value.(*models.Object_File)
				if ok {
					curFile = *of.File
				}
			}

			newFile, err = fn(curFile)
			if err != nil {
				return nil, err
			}
			return &models.Object{
				Value: &models.Object_File{
					File: newFile,
				},
			}, nil
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

func fileAppend(ctx context.Context, s ReadWriteOnce, x models.File, p []byte) (*models.File, error) {
	var err error
	y := &models.File{}

	ref, err := s.Post(ctx, p)
	if err != nil {
		return nil, err
	}

	offset := x.Size
	key := offset2Key(offset)
	y.Tree, err = x.Tree.Put(ctx, s, key[:], ref)
	if err != nil {
		return nil, err
	}
	y.Size = x.Size + uint64(len(p))
	return y, nil
}

func fileReadAt(ctx context.Context, s ReadWriteOnce, x models.File, offset uint64, p []byte) (n int, err error) {
	for n < len(p) {
		ent, err := x.Tree.MaxLteq(ctx, s, offset2Key(offset))
		if err != nil {
			return 0, err
		}
		if ent == nil {
			// done, no entry for this offset, empty file
			return n, io.EOF
		}

		o := key2Offset(ent.Key)
		if offset < o {
			return 0, errors.New("got wrong entry from tree")
		}
		relo := offset - o
		data, err := s.Get(ctx, *ent.Ref)
		if err != nil {
			return n, err
		}

		if int(relo) >= len(data) {
			return n, io.EOF
		}
		n2 := copy(p[n:], data[relo:])
		n += n2
		offset += uint64(n2)
	}

	return n, nil
}

func offset2Key(x uint64) []byte {
	keyBuf := [8]byte{}
	binary.BigEndian.PutUint64(keyBuf[:], x)
	return keyBuf[:]
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
		cont = f(*entry.Ref)
	}
	return cont, nil
}
