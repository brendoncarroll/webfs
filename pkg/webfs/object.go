package webfs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Path []string

func (p Path) String() string {
	return strings.Join(p, "/")
}

func parsePath(x string) Path {
	y := []string{}
	for _, part := range strings.Split(x, "/") {
		if part != "" {
			y = append(y, part)
		}
	}
	return Path(y)
}

type Object interface {
	Find(ctx context.Context, p Path, objs []Object) ([]Object, error)
	Lookup(ctx context.Context, p Path) (Object, error)
	Walk(ctx context.Context, f func(Object) bool) (bool, error)
	Path() Path
	Size() uint64
	String() string

	RefIter(ctx context.Context, f func(webref.Ref) bool) (bool, error)

	getFS() *WebFS
	getStore() *Store
	getOptions() *Options
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

func (o *baseObject) getFS() *WebFS {
	return o.fs
}

func (o *baseObject) getStore() *Store {
	return o.parent.getStore()
}

func (o *baseObject) getOptions() *Options {
	return o.parent.getOptions()
}

func unmarshalObject(parent Object, name string, data []byte) (Object, error) {
	m := models.Object{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return wrapObject(parent, name, &m)
}

func wrapObject(parent Object, nameInParent string, o *models.Object) (Object, error) {
	base := baseObject{
		parent:       parent,
		nameInParent: nameInParent,
		fs:           parent.getFS(),
	}

	switch o2 := o.Value.(type) {
	case *models.Object_Volume:
		wfs := parent.getFS()
		cell, err := wfs.getCellBySpec(o2.Volume)
		if err != nil {
			return nil, err
		}
		return &Volume{
			cell:       cell,
			baseObject: base,
		}, nil

	case *models.Object_File:
		return &File{
			baseObject: base,
			m:          *o2.File,
		}, nil

	case *models.Object_Dir:
		return &Dir{
			m:          *o2.Dir,
			baseObject: base,
		}, nil

	default:
		return nil, errors.New("Invalid Object")
	}
}
