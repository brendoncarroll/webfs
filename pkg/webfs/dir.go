package webfs

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/webref"
	"github.com/brendoncarroll/webfs/pkg/wrds"
)

type DirMutator func(ctx context.Context, cur *models.Dir) (*models.Dir, error)

type DirEntry struct {
	Name   string
	Object Object
}

type Dir struct {
	m models.Dir
	baseObject
}

func newDir(ctx context.Context, parent Object, name string) (*Dir, error) {
	if parent == nil {
		return nil, errors.New("dir must have parent")
	}
	dir := &Dir{
		baseObject: baseObject{
			parent:       parent,
			nameInParent: name,
		},
	}
	err := dir.apply(ctx, func(ctx context.Context, cur *models.Dir) (*models.Dir, error) {
		if cur != nil {
			return nil, errors.New("already exists")
		}
		m := &models.Dir{Tree: wrds.NewTree()}
		return m, nil
	})
	if err != nil {
		return nil, err
	}
	return dir, nil
}

func (d *Dir) Find(ctx context.Context, p Path, objs []Object) ([]Object, error) {
	if len(p) == 0 {
		return append(objs, d), nil
	}
	ent, err := d.Get(ctx, p[0])
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, nil
	}
	o, err := wrapObject(d, ent.Name, ent.Object)
	if err != nil {
		return nil, err
	}
	return o.Find(ctx, p[1:], objs)
}

func (d *Dir) Lookup(ctx context.Context, p Path) (Object, error) {
	objs, err := d.Find(ctx, p, []Object{})
	if err != nil {
		return nil, err
	}
	if len(objs) < 1 {
		return nil, nil
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

		dirEnt := models.DirEntry{}
		if err := webref.Load(ctx, d.getStore(), *ent.Ref, &dirEnt); err != nil {
			return false, err
		}

		name := string(ent.Key)
		o2, err := wrapObject(d, name, dirEnt.Object)
		if err != nil {
			return false, err
		}
		cont = f(o2)
		if cont {
			cont, err = o2.Walk(ctx, f)
			if err != nil {
				return false, err
			}
		}
	}
	return cont, nil
}

func (d *Dir) Get(ctx context.Context, name string) (*models.DirEntry, error) {
	return dirGet(ctx, d.getStore(), d.m, name)
}

func (d *Dir) Put(ctx context.Context, ent models.DirEntry) error {
	return d.apply(ctx, func(ctx context.Context, cur *models.Dir) (*models.Dir, error) {
		return dirPut(ctx, d.getStore(), *d.getOptions().DataOpts, *cur, ent)
	})
}

func (d *Dir) Delete(ctx context.Context, name string) error {
	return d.apply(ctx, func(ctx context.Context, cur *models.Dir) (*models.Dir, error) {
		return dirDelete(ctx, d.getStore(), *d.getOptions().DataOpts, *cur, name)
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

		dirEnt := &models.DirEntry{}
		if err := webref.Load(ctx, d.getStore(), *ent.Ref, dirEnt); err != nil {
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
	return "Object{Dir}"
}

func (d *Dir) dirSplit(ctx context.Context, s stores.ReadWriteOnce, opts webref.Options, x models.Dir) (*models.Dir, error) {
	newTree, err := x.Tree.Split(ctx, s, opts)
	if err != nil {
		return nil, err
	}
	return &models.Dir{Tree: newTree}, nil
}

func (d *Dir) put(ctx context.Context, name string, fn ObjectMutator) error {
	store := d.getStore()
	opts := d.getOptions().DataOpts
	return d.apply(ctx, func(ctx context.Context, cur *models.Dir) (*models.Dir, error) {
		// get the entry at name
		ent, err := dirGet(ctx, store, *cur, name)
		if err != nil {
			return nil, err
		}
		var o *models.Object
		if ent != nil {
			o = ent.Object
		}
		// mutate the object
		o2, err := fn(o)
		if err != nil {
			return nil, err
		}
		// replace the entry at name
		newEnt := models.DirEntry{Name: name, Object: o2}
		return dirPut(ctx, store, *opts, *cur, newEnt)
	})
}

func (d *Dir) apply(ctx context.Context, fn DirMutator) error {
	var (
		put    func(context.Context, ObjectMutator) error
		newDir *models.Dir
	)

	switch x := d.parent.(type) {
	case *Volume:
		put = x.put
	case *Dir:
		put = func(ctx context.Context, fn ObjectMutator) error {
			return x.put(ctx, d.nameInParent, fn)
		}
	default:
		log.Printf("value: %v type: %T\n", d.parent, d.parent)
		panic("invalid parent")
	}

	err := put(ctx, func(cur *models.Object) (*models.Object, error) {
		var curDir *models.Dir
		if cur != nil {
			od, ok := cur.Value.(*models.Object_Dir)
			if ok {
				curDir = od.Dir
			}
		}

		var err error
		newDir, err = fn(ctx, curDir)
		if err != nil {
			newDir = nil
			return nil, err
		}
		return &models.Object{
			Value: &models.Object_Dir{
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

func dirGet(ctx context.Context, store stores.Read, m models.Dir, name string) (*models.DirEntry, error) {
	key := []byte(name)
	treeEnt, err := m.Tree.Get(ctx, store, key)
	if err != nil {
		return nil, err
	}
	if treeEnt == nil {
		return nil, nil
	}

	dirEnt := models.DirEntry{}
	if err := webref.Load(ctx, store, *treeEnt.Ref, &dirEnt); err != nil {
		return nil, err
	}
	return &dirEnt, nil
}

func dirPut(ctx context.Context, store stores.ReadWriteOnce, opts webref.Options, m models.Dir, ent models.DirEntry) (*models.Dir, error) {
	if strings.Contains(ent.Name, "/") {
		return nil, errors.New("directory name contains a slash")
	}

	var (
		ref *webref.Ref
		err error
	)
	for {
		ref, err = webref.Store(ctx, store, opts, &ent)
		if err == webref.ErrMaxSizeExceeded {
			ent, err = dirEntSplit(ctx, store, opts, ent)
			if err != nil {
				return nil, err
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}

	key := []byte(ent.Name)
	tree, err := m.Tree.Put(ctx, store, opts, key, ref)
	if err != nil {
		return nil, err
	}
	return &models.Dir{Tree: tree}, nil
}

func dirDelete(ctx context.Context, store stores.ReadWriteOnce, opts webref.Options, m models.Dir, name string) (*models.Dir, error) {
	key := []byte(name)
	newTree, err := m.Tree.Delete(ctx, store, opts, key)
	if err != nil {
		return nil, err
	}
	return &models.Dir{Tree: newTree}, nil
}

func dirSplit(ctx context.Context, store stores.ReadWriteOnce, opts webref.Options, x models.Dir) (*models.Dir, error) {
	newTree, err := x.Tree.Split(ctx, store, opts)
	if err != nil {
		return nil, err
	}
	return &models.Dir{Tree: newTree}, nil
}

func dirEntSplit(ctx context.Context, store stores.ReadWriteOnce, opts webref.Options, x models.DirEntry) (models.DirEntry, error) {
	o2, err := objectSplit(ctx, store, opts, *x.Object)
	if err != nil {
		return models.DirEntry{}, nil
	}
	return models.DirEntry{Object: o2, Name: x.Name}, nil
}
