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
		return newVolume(ctx, o2.Volume, parent.getFS(), parent, nameInParent)

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
