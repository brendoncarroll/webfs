package webfs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
	"github.com/brendoncarroll/webfs/pkg/wrds"
)

type FileMutator func(cur webfsim.File) (*webfsim.File, error)

type File struct {
	m webfsim.File

	baseObject
}

func newFile(ctx context.Context, parent Object, name string) (*File, error) {
	f := &File{
		m: webfsim.File{Tree: wrds.NewTree()},
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

	opts := f.getOptions()
	ctx = webref.SetCodecCtx(ctx, opts.DataOpts.Codec)

	m, err := webfsim.FileFromReader(ctx, store, r)
	if err != nil {
		return err
	}

	// This is a total replace not a modification of existing content.
	return f.Apply(ctx, func(cur webfsim.File) (*webfsim.File, error) {
		return m, nil
	})
}

func (f *File) GetAtPath(ctx context.Context, p Path, objs []Object) ([]Object, error) {
	if len(p) == 0 {
		objs = append(objs, f)
		return objs, nil
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
	return webfsim.FileReadAt(ctx, f.getStore(), f.m, offset, p)
}

func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	ctx := context.TODO()
	opts := f.getOptions()
	ctx = webref.SetCodecCtx(ctx, opts.DataOpts.Codec)

	err = f.Apply(ctx, func(x webfsim.File) (*webfsim.File, error) {
		return &x, errors.New("File.WriteAt not implemented")
	})
	if err != nil {
		return 0, err
	}
	return n, err
}

func (f *File) Append(p []byte) error {
	ctx := context.TODO()
	newFile, err := webfsim.FileAppend(ctx, f.getStore(), f.m, p)
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
	return "File{}"
}

func (f File) FileInfo() FileInfo {
	t := time.Now().AddDate(0, 0, -1)
	return FileInfo{
		CreatedAt:  t,
		ModifiedAt: t,
		AccessedAt: t,
		Mode:       0644,
		Size:       f.Size(),
	}
}

func (f *File) RefIter(ctx context.Context, fn func(webref.Ref) bool) (bool, error) {
	return refIterTree(ctx, f.getStore(), f.m.Tree, fn)
}

func (f *File) Apply(ctx context.Context, fn FileMutator) error {
	var (
		newFile *webfsim.File
		err     error
	)

	switch x := f.parent.(type) {
	case *Dir:
		err = x.put(ctx, f.nameInParent, func(cur *webfsim.Object) (*webfsim.Object, error) {
			curFile := f.m
			if cur != nil {
				of, ok := cur.Value.(*webfsim.Object_File)
				if ok {
					curFile = *of.File
				}
			}

			newFile, err = fn(curFile)
			if err != nil {
				return nil, err
			}
			return &webfsim.Object{
				Value: &webfsim.Object_File{
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

func refIterTree(ctx context.Context, store webref.Getter, t *wrds.Tree, f func(webref.Ref) bool) (bool, error) {
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
