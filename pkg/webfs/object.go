package webfs

import (
	"context"
	"errors"

	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Object interface {
	GetAtPath(ctx context.Context, p Path, objs []Object) ([]Object, error)
	Walk(ctx context.Context, f func(Object) bool) (bool, error)
	Path() Path
	Size() uint64
	String() string
	RefIter(ctx context.Context, f func(webref.Ref) bool) (bool, error)

	getFS() *WebFS
	getStore() *Store
	getOptions() *Options
	getParent() Object
}

type baseObject struct {
	fs *WebFS

	parent       Object
	nameInParent string
}

func (o *baseObject) Path() Path {
	p := o.parent.Path()
	if o.nameInParent != "" {
		p = append(p, o.nameInParent)
	}
	return p
}

func (o *baseObject) getParent() Object {
	return o.parent
}

func (o *baseObject) getFS() *WebFS {
	return o.fs
}

func (o *baseObject) getStore() *Store {
	return o.parent.getStore()
}

func (o *baseObject) getOptions() *Options {
	return o.parent.getOptions()
}

func wrapObject(parent Object, nameInParent string, o *webfsim.Object) (Object, error) {
	base := baseObject{
		parent:       parent,
		nameInParent: nameInParent,
		fs:           parent.getFS(),
	}

	switch o2 := o.Value.(type) {
	case *webfsim.Object_Volume:
		ctx := context.TODO()
		return setupVolume(ctx, o2.Volume, parent.getFS(), parent, nameInParent)

	case *webfsim.Object_File:
		return &File{
			baseObject: base,
			m:          *o2.File,
		}, nil

	case *webfsim.Object_Dir:
		return &Dir{
			m:          *o2.Dir,
			baseObject: base,
		}, nil

	default:
		return nil, errors.New("Invalid Object")
	}
}

func putAt(ctx context.Context, parent Object, o *webfsim.Object, name string) error {
	var put func(context.Context, ObjectMutator) error
	switch x := parent.(type) {
	case *Volume:
		if name != "" {
			return errors.New("volumes do not support entries")
		}
		put = x.put
	case *Dir:
		put = func(ctx context.Context, fn ObjectMutator) error {
			return x.put(ctx, name, fn)
		}
	default:
		panic("lookup returned file")
	}
	err := put(ctx, func(cur *webfsim.Object) (*webfsim.Object, error) {
		if cur != nil {
			return nil, errors.New("object already here")
		}
		return o, nil
	})
	return err
}

func containingVolume(o Object) *Volume {
	for {
		par := o.getParent()
		if par == nil {
			break
		}
		v, ok := par.(*Volume)
		if ok {
			return v
		}
		o = par
	}
	return nil
}
