package webfs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/webfs/models"
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

	RefIter(ctx context.Context, f func(Ref) bool) (bool, error)

	getFS() *WebFS
	getStore() ReadWriteOnce
	getOptions() Options
}

type baseObject struct {
	fs    *WebFS
	store ReadWriteOnce

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

func (o *baseObject) getStore() ReadWriteOnce {
	return o.store
}

func (o *baseObject) getOptions() Options {
	return o.parent.getOptions()
}

func unmarshalObject(parent Object, name string, data []byte) (Object, error) {
	m := models.Object{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return wrapObject(parent, name, m)
}

func wrapObject(parent Object, nameInParent string, o models.Object) (Object, error) {
	base := baseObject{
		parent:       parent,
		nameInParent: nameInParent,
		store:        parent.getStore(),
		fs:           parent.getFS(),
	}

	switch {
	case o.Cell != nil:
		wfs := parent.getFS()
		cell := wfs.getCellBySpec(*o.Cell)
		return &Volume{
			cell:       cell,
			baseObject: base,
		}, nil
	case o.File != nil:
		return &File{
			baseObject: base,
			m:          *o.File,
		}, nil
	case o.Dir != nil:
		return &Dir{
			m:          *o.Dir,
			baseObject: base,
		}, nil
	default:
		return nil, errors.New("Invalid Object")
	}
}

func emptyObject(o models.Object) bool {
	for _, x := range []interface{}{
		o.Cell,
		o.File,
		o.Dir,
		o.Snapshot,
	} {
		if x != nil {
			return false
		}
	}
	return true
}
