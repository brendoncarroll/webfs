package webfs

import (
	"context"
	"errors"

	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Object interface {
	GetAtPath(ctx context.Context, p Path, objs []Object, max int) ([]Object, error)
	Walk(ctx context.Context, f func(Object) bool) (bool, error)
	RefIter(ctx context.Context, f func(webref.Ref) bool) (bool, error)

	Path() Path
	Size() uint64
	String() string
	Describe() string

	getFS() *WebFS
	getStore() *Store
	getOptions() *Options
	getParent() Object
	getName() string
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

func (o *baseObject) getName() string {
	return o.nameInParent
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

func apply(ctx context.Context, parent Object, name string, f ObjectMutator) error {
	switch x := parent.(type) {
	case *Volume:
		if name != "" {
			return errors.New("volumes do not support entries")
		}
		return x.ApplyObject(ctx, f)
	case *Dir:
		return x.ApplyEntry(ctx, name, f)
	default:
		panic("can't apply")
	}
}

func putAt(ctx context.Context, parent Object, name string, o *webfsim.Object) error {
	return apply(ctx, parent, name, func(x *webfsim.Object) (*webfsim.Object, error) {
		return o, nil
	})
}

func deleteAt(ctx context.Context, parent Object, name string) error {
	switch x := parent.(type) {
	case *Volume:
		if name != "" {
			return errors.New("volumes do not support entries")
		}
		return x.ApplyObject(ctx, func(x *webfsim.Object) (*webfsim.Object, error) {
			return nil, nil
		})
	case *Dir:
		return x.Delete(ctx, name)
	default:
		panic("can't delete")
	}
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
