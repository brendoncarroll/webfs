package webfs

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
	"github.com/brendoncarroll/webfs/pkg/wrds"
	"github.com/golang/protobuf/jsonpb"
)

type DirMutator func(cur *webfsim.Dir) (*webfsim.Dir, error)

func IdentityDM(ctx context.Context, cur *webfsim.Dir) (*webfsim.Dir, error) {
	return cur, nil
}

type DirEntry struct {
	Name   string
	Object Object
}

type Dir struct {
	m webfsim.Dir
	baseObject
}

func NewDir(ctx context.Context, parent Object, name string) (*Dir, error) {
	if parent == nil {
		return nil, errors.New("dir must have parent")
	}
	dir := &Dir{
		baseObject: baseObject{
			parent:       parent,
			nameInParent: name,
		},
	}
	err := dir.Apply(ctx, func(cur *webfsim.Dir) (*webfsim.Dir, error) {
		if cur != nil {
			return nil, errors.New("already exists")
		}
		m := &webfsim.Dir{Tree: wrds.NewTree()}
		return m, nil
	})
	if err != nil {
		return nil, err
	}
	return dir, nil
}

func (d *Dir) GetAtPath(ctx context.Context, p Path, objs []Object, max int) ([]Object, error) {
	if len(p) == 0 {
		return append(objs, d), nil
	}
	o, err := d.Get(ctx, p[0])
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, nil
	}
	return o.GetAtPath(ctx, p[1:], objs, max)
}

func (d *Dir) Lookup(ctx context.Context, p Path) (Object, error) {
	objs, err := d.GetAtPath(ctx, p, []Object{}, -1)
	if err != nil {
		return nil, err
	}
	if len(objs) < 1 {
		return nil, ErrNotExist
	}
	return objs[len(objs)-1], nil
}

func (d *Dir) Walk(ctx context.Context, f func(Object) bool) (bool, error) {
	cont := f(d)

	if !cont {
		return cont, nil
	}

	iter, err := d.m.Tree.Iterate(ctx, d.getStore(), nil)
	if err != nil {
		return false, nil
	}
	for cont {
		ent, err := iter.Next(ctx)
		if err != nil {
			return false, err
		}
		if ent == nil {
			break
		}

		dirEnt := webfsim.DirEntry{}
		if err := webref.GetAndDecode(ctx, d.getStore(), *ent.Ref, &dirEnt); err != nil {
			return false, err
		}

		name := string(ent.Key)
		o2, err := wrapObject(d, name, dirEnt.Object)
		if err != nil {
			return false, err
		}
		cont, err = o2.Walk(ctx, f)
		if err != nil {
			return false, err
		}
	}
	return cont, nil
}

func (d *Dir) Get(ctx context.Context, name string) (Object, error) {
	x, err := webfsim.DirGet(ctx, d.getStore(), d.m, name)
	if err != nil {
		return nil, err
	}
	if x == nil {
		return nil, nil
	}
	return wrapObject(d, name, x.Object)
}

func (d *Dir) Put(ctx context.Context, ent webfsim.DirEntry) error {
	opts := d.getOptions()
	ctx = webref.SetCodecCtx(ctx, opts.DataOpts.Codec)

	return d.Apply(ctx, func(cur *webfsim.Dir) (*webfsim.Dir, error) {
		return webfsim.DirPut(ctx, d.getStore(), *cur, ent)
	})
}

func (d *Dir) Delete(ctx context.Context, name string) error {
	opts := d.getOptions()
	ctx = webref.SetCodecCtx(ctx, opts.DataOpts.Codec)

	return d.Apply(ctx, func(cur *webfsim.Dir) (*webfsim.Dir, error) {
		return webfsim.DirDelete(ctx, d.getStore(), *cur, name)
	})
}

func (d *Dir) Entries(ctx context.Context) ([]DirEntry, error) {
	entries := []DirEntry{}
	iter, err := d.m.Tree.Iterate(ctx, d.getStore(), nil)
	if err != nil {
		return nil, err
	}

	for {
		ent, err := iter.Next(ctx)
		if err != nil {
			return nil, err
		}
		if ent == nil {
			break
		}

		dirEnt := &webfsim.DirEntry{}
		if err := webref.GetAndDecode(ctx, d.getStore(), *ent.Ref, dirEnt); err != nil {
			return nil, err
		}
		o, err := wrapObject(d, dirEnt.Name, dirEnt.Object)
		if err != nil {
			return nil, err
		}
		entries = append(entries, DirEntry{Name: dirEnt.Name, Object: o})
	}

	return entries, nil
}

func (d *Dir) RefIter(ctx context.Context, f func(webref.Ref) bool) (bool, error) {
	return refIterTree(ctx, d.getStore(), d.m.Tree, f)
}

func (d *Dir) Size() uint64 {
	return 0
}

func (d *Dir) String() string {
	return "Dir{}"
}

func (d Dir) FileInfo() FileInfo {
	t := time.Now().AddDate(0, 0, -1)
	return FileInfo{
		CreatedAt:  t,
		ModifiedAt: t,
		AccessedAt: t,
		Mode:       0755 | os.ModeDir,
	}
}

func (d Dir) FS() *WebFS {
	return d.getFS()
}

func (d *Dir) split(ctx context.Context, s webref.Poster, x webfsim.Dir) (*webfsim.Dir, error) {
	newTree, err := x.Tree.Split(ctx, s)
	if err != nil {
		return nil, err
	}
	return &webfsim.Dir{Tree: newTree}, nil
}

func (d *Dir) ApplyEntry(ctx context.Context, name string, fn ObjectMutator) error {
	store := d.getStore()
	opts := d.getOptions()
	ctx = webref.SetCodecCtx(ctx, opts.DataOpts.Codec)

	return d.Apply(ctx, func(cur *webfsim.Dir) (*webfsim.Dir, error) {
		// get the entry at name
		ent, err := webfsim.DirGet(ctx, store, *cur, name)
		if err != nil {
			return nil, err
		}
		var o *webfsim.Object
		if ent != nil {
			o = ent.Object
		}
		// mutate the object
		o2, err := fn(o)
		if err != nil {
			return nil, err
		}
		// replace the entry at name
		newEnt := webfsim.DirEntry{Name: name, Object: o2}
		return webfsim.DirPut(ctx, store, *cur, newEnt)
	})
}

func (d *Dir) Describe() string {
	m := jsonpb.Marshaler{
		Indent: " ",
	}
	o := &webfsim.Object{
		Value: &webfsim.Object_Dir{&d.m},
	}
	s, err := m.MarshalToString(o)
	if err != nil {
		panic(err)
	}
	return s
}

func (d *Dir) Apply(ctx context.Context, fn DirMutator) error {
	var newDir *webfsim.Dir

	err := apply(ctx, d.parent, d.nameInParent, func(cur *webfsim.Object) (*webfsim.Object, error) {
		var curDir *webfsim.Dir
		if cur != nil {
			od, ok := cur.Value.(*webfsim.Object_Dir)
			if ok {
				curDir = od.Dir
			}
		}

		var err error
		newDir, err = fn(curDir)
		if err != nil {
			newDir = nil
			return nil, err
		}
		return &webfsim.Object{
			Value: &webfsim.Object_Dir{
				Dir: newDir,
			},
		}, nil
	})
	if err != nil {
		return err
	}
	if newDir != nil {
		d.m = *newDir
	}
	return nil
}

func (d *Dir) Sync(ctx context.Context) error {
	return d.Apply(ctx, func(cur *webfsim.Dir) (*webfsim.Dir, error) {
		return cur, nil
	})
}
