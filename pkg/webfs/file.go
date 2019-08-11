package webfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/webref"
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

	store := f.getStore()
	opts := *f.getOptions().DataOpts
	buf := make([]byte, f.getStore().MaxBlobSize())
	if len(buf) == 0 {
		panic("max blob size 0")
	}

	size := uint64(0)
	tb := wrds.NewTreeBuilder()
	for {
		n, err := r.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		data := buf[:n]

		ref, err := webref.Post(ctx, store, opts, data)
		if err != nil {
			return err
		}
		ent := &wrds.TreeEntry{
			Key: offset2Key(size),
			Ref: ref,
		}
		if err = tb.Put(ctx, store, opts, ent); err != nil {
			return err
		}
		size += uint64(n)
	}
	tree, err := tb.Finish(ctx, store, opts)
	if err != nil {
		return err
	}
	m := &models.File{
		Tree: tree,
		Size: size,
	}

	// This is a total replace not a modification of existing content.
	return f.Apply(ctx, func(cur models.File) (*models.File, error) {
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
	return fileReadAt(ctx, f.getStore(), f.m, offset, p)
}

func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	ctx := context.TODO()
	err = f.Apply(ctx, func(x models.File) (*models.File, error) {
		return &x, errors.New("File.WriteAt not implemented")
	})
	if err != nil {
		return 0, err
	}
	return n, err
}

func (f *File) Append(p []byte) error {
	ctx := context.TODO()
	newFile, err := fileAppend(ctx, f.getStore(), *f.getOptions().DataOpts, f.m, p)
	if err != nil {
		return err
	}
	f.m = *newFile
	return nil
}

func (f *File) Size() uint64 {
	return f.m.Size
}

func (f File) String() string {
	return "Object{File}"
}

func (f *File) RefIter(ctx context.Context, fn func(webref.Ref) bool) (bool, error) {
	return refIterTree(ctx, f.getStore(), f.m.Tree, fn)
}

func (f *File) Apply(ctx context.Context, fn FileMutator) error {
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

func fileAppend(ctx context.Context, s stores.ReadWriteOnce, opts webref.Options, x models.File, p []byte) (*models.File, error) {
	var (
		err error
	)
	y := &models.File{}

	ref, err := webref.Post(ctx, s, opts, p)
	if err != nil {
		return nil, err
	}

	offset := x.Size
	key := offset2Key(offset)
	y.Tree, err = x.Tree.Put(ctx, s, opts, key[:], ref)
	if err != nil {
		return nil, err
	}
	y.Size = x.Size + uint64(len(p))
	return y, nil
}

func fileReadAt(ctx context.Context, s stores.ReadWriteOnce, x models.File, offset uint64, p []byte) (n int, err error) {
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
		data, err := webref.Get(ctx, s, *ent.Ref)
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

func fileSplit(ctx context.Context, store stores.ReadWriteOnce, opts webref.Options, x models.File) (*models.File, error) {
	newTree, err := x.Tree.Split(ctx, store, opts)
	if err != nil {
		return nil, err
	}
	y := x
	y.Tree = newTree
	return &y, nil
}

func offset2Key(x uint64) []byte {
	keyBuf := [8]byte{}
	binary.BigEndian.PutUint64(keyBuf[:], x)
	return keyBuf[:]
}

func key2Offset(x []byte) uint64 {
	return binary.BigEndian.Uint64(x)
}

func refIterTree(ctx context.Context, store stores.Read, t *wrds.Tree, f func(webref.Ref) bool) (bool, error) {
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
			return true, nil
		}
		cont = f(*entry.Ref)
	}
	return cont, nil
}
