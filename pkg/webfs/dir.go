package webfs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/webfs/models"
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
	dir := &Dir{
		baseObject: baseObject{
			parent:       parent,
			store:        parent.getStore(),
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

	iter, err := d.m.Tree.Iterate(ctx, d.store, nil)
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
		data, err := d.store.Get(ctx, ent.Ref)
		if err != nil {
			return false, err
		}
		name := string(ent.Key)
		o, err := unmarshalObject(d, name, data)
		if err != nil {
			return false, nil
		}
		cont = f(o)
	}
	return cont, nil
}

func (d *Dir) Get(ctx context.Context, name string) (*models.DirEntry, error) {
	return dirGet(ctx, d.store, d.m, name)
}

func (d *Dir) Put(ctx context.Context, ent models.DirEntry) error {
	return d.apply(ctx, func(ctx context.Context, cur *models.Dir) (*models.Dir, error) {
		return dirPut(ctx, d.store, *cur, ent)
	})
}

func (d *Dir) Delete(ctx context.Context, name string) error {
	return d.apply(ctx, func(ctx context.Context, cur *models.Dir) (*models.Dir, error) {
		return dirDelete(ctx, d.store, *cur, name)
	})
}

func (d *Dir) Entries(ctx context.Context) ([]DirEntry, error) {
	entries := []DirEntry{}
	iter, err := d.m.Tree.Iterate(ctx, d.store, nil)
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
		data, err := d.store.Get(ctx, ent.Ref)
		if err != nil {
			return nil, err
		}
		dirEnt := models.DirEntry{}
		if err := json.Unmarshal(data, &dirEnt); err != nil {
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

func (d *Dir) RefIter(ctx context.Context, f func(Ref) bool) (bool, error) {
	return refIterTree(ctx, d.store, d.m.Tree, f)
}

func (d *Dir) Size() uint64 {
	return 0
}

func (d *Dir) String() string {
	return "Object{Dir}"
}

// func (d *Dir) split(ctx context.Context, s ReadWriteOnce) error {
// 	newTree, err := d.m.Tree.Split(ctx, s)
// 	if err != nil {
// 		return err
// 	}
// 	d.m = models.Dir{Tree: newTree}
// 	return nil
// }

func (d *Dir) put(ctx context.Context, name string, fn ObjectMutator) error {
	store := d.store.(ReadWriteOnce)
	return d.apply(ctx, func(ctx context.Context, cur *models.Dir) (*models.Dir, error) {
		// get the entry at name
		ent, err := dirGet(ctx, d.store, *cur, name)
		if err != nil {
			return nil, err
		}
		var o *models.Object
		if ent != nil {
			o = &ent.Object
		}
		// mutate the object
		o2, err := fn(o)
		if err != nil {
			return nil, err
		}
		// replace the entry at name
		newEnt := models.DirEntry{Name: name, Object: *o2}
		return dirPut(ctx, store, *cur, newEnt)
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
		panic("invalid parent")
	}

	err := put(ctx, func(cur *models.Object) (*models.Object, error) {
		var curDir *models.Dir
		if cur != nil && cur.Dir != nil {
			curDir = cur.Dir
		}
		var err error
		newDir, err = fn(ctx, curDir)
		if err != nil {
			newDir = nil
			return nil, err
		}
		return &models.Object{Dir: newDir}, nil
	})
	if err != nil {
		return err
	}
	if newDir != nil {
		d.m = *newDir
	}
	return nil
}

func dirGet(ctx context.Context, store Read, m models.Dir, name string) (*models.DirEntry, error) {
	key := []byte(name)
	treeEnt, err := m.Tree.Get(ctx, store, key)
	if err != nil {
		return nil, err
	}
	if treeEnt == nil {
		return nil, nil
	}
	data, err := store.Get(ctx, treeEnt.Ref)
	if err != nil {
		return nil, err
	}
	dirEnt := models.DirEntry{}
	if err := json.Unmarshal(data, &dirEnt); err != nil {
		return nil, err
	}
	return &dirEnt, nil
}

func dirPut(ctx context.Context, store ReadWriteOnce, m models.Dir, ent models.DirEntry) (*models.Dir, error) {
	if strings.Contains(ent.Name, "/") {
		return nil, errors.New("directory name contains a slash")
	}
	store, ok := store.(ReadWriteOnce)
	if !ok {
		return nil, errors.New("Need writable store")
	}

	data, _ := json.Marshal(ent)
	entRef, err := store.Post(ctx, data)
	if err != nil {
		return nil, err
	}
	key := []byte(ent.Name)
	tree, err := m.Tree.Put(ctx, store, key, *entRef)
	if err != nil {
		return nil, err
	}
	return &models.Dir{Tree: tree}, nil
}

func dirDelete(ctx context.Context, store ReadWriteOnce, m models.Dir, name string) (*models.Dir, error) {
	key := []byte(name)
	newTree, err := m.Tree.Delete(ctx, store, key)
	if err != nil {
		return nil, err
	}
	return &models.Dir{Tree: newTree}, nil
}
