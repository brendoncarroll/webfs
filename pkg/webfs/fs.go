package webfs

import (
	"context"
	"errors"
	"io"
	"log"
	"path"
	"sync"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
)

type WebFS struct {
	root  Volume
	store stores.ReadWriteOnce
	cells sync.Map
}

func New(rootCell Cell, store stores.ReadWriteOnce) (*WebFS, error) {
	wfs := &WebFS{
		store: store,
		cells: sync.Map{},
	}
	wfs.cells.Store("", rootCell)
	wfs.addCell(rootCell)

	rv := Volume{
		cell: rootCell,
		baseObject: baseObject{
			fs:           wfs,
			store:        nil,
			parent:       nil,
			nameInParent: "",
		},
	}
	wfs.root = rv

	return wfs, nil
}

func (wfs *WebFS) Lookup(ctx context.Context, p string) (Object, error) {
	p2 := parsePath(p)
	o, err := wfs.root.Lookup(ctx, p2)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errors.New("no entry at " + p)
	}
	return o, nil
}

func (wfs *WebFS) Ls(ctx context.Context, p string) ([]DirEntry, error) {
	o, err := wfs.Lookup(ctx, p)
	if err != nil {
		return nil, err
	}
	dir, ok := o.(*Dir)
	if !ok {
		log.Println(o)
		return nil, errors.New("cannot ls non-dir")
	}
	entries, err := dir.Entries(ctx)
	return entries, err
}

func (wfs *WebFS) ImportFile(ctx context.Context, r io.Reader, dst string) error {
	dirp := dirpath(dst)
	basep := basepath(dst)

	parent, err := wfs.Lookup(ctx, dirp)
	if err != nil {
		return err
	}
	f, err := newFile(ctx, parent, basep)
	if err != nil {
		return err
	}
	if err := f.SetData(ctx, r); err != nil {
		return err
	}
	return nil
}

func (wfs *WebFS) Mkdir(ctx context.Context, p string) error {
	dirp := dirpath(p)
	basep := basepath(p)

	parent, err := wfs.Lookup(ctx, dirp)
	if err != nil {
		return err
	}
	_, err = newDir(ctx, parent, basep)
	if err != nil {
		return err
	}
	return err
}

func (wfs *WebFS) Cat(ctx context.Context, p string) (io.ReadCloser, error) {
	o, err := wfs.Lookup(ctx, p)
	if err != nil {
		return nil, err
	}
	f, ok := o.(*File)
	if !ok {
		return nil, errors.New("can't cat non-file")
	}
	fh := f.GetHandle()
	return fh, nil
}

func (wfs *WebFS) OpenFile(ctx context.Context, p string) (*FileHandle, error) {
	o, err := wfs.Lookup(ctx, p)
	if err != nil {
		return nil, err
	}
	f, ok := o.(*File)
	if !ok {
		return nil, errors.New("Cannot open non-file")
	}
	fh := f.GetHandle()
	return fh, nil
}

func (wfs *WebFS) Delete(ctx context.Context, p string) error {
	dirp := path.Dir(p)
	basep := path.Base(p)
	if basep == p {
		return errors.New("cannot delete " + p)
	}

	o, err := wfs.Lookup(ctx, dirp)
	if err != nil {
		return err
	}
	d, ok := o.(*Dir)
	if !ok {
		return errors.New("cannot delete from non-dir parent")
	}
	return d.Delete(ctx, basep)
}

func (wfs *WebFS) WalkObjects(ctx context.Context, f func(o Object) bool) error {
	v := wfs.root
	_, err := v.Walk(ctx, f)
	return err
}

func (wfs *WebFS) RefIter(ctx context.Context, f func(ref Ref) bool) error {
	v := wfs.root
	var topErr error

	_, err := v.Walk(ctx, func(o Object) bool {
		cont, err := o.RefIter(ctx, f)
		if err != nil {
			topErr = err
			return false
		}
		return cont
	})
	if err != nil {
		return err
	}
	return topErr
}

func (wfs *WebFS) addCell(cell Cell) {
	_, loaded := wfs.cells.LoadOrStore(cell.ID(), cell)
	if loaded {
		log.Println("loaded cell", cell)
	}
}

func (wfs *WebFS) getCellByID(id string) Cell {
	cell, _ := wfs.cells.Load(id)
	if cell == nil {
		return nil
	}
	return cell.(Cell)
}

func (wfs *WebFS) getCellBySpec(spec models.CellSpec) Cell {
	newCell := cells.Make(spec)
	cell, loaded := wfs.cells.LoadOrStore(newCell.ID(), newCell)
	if loaded {
		log.Println("loadeded new cell", newCell.ID())
	}
	if cell == nil {
		return nil
	}
	return cell.(Cell)
}

// func (wfs *WebFS) loadObject(ctx context.Context, ref webref.Ref) (*Object, error) {
// 	obj := &Object{}
// 	data, err := ref.Deref(ctx, wfs.store)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if err := obj.Unmarshal(data); err != nil {
// 		return nil, err
// 	}
// 	return obj, nil
// }

// func (wfs *WebFS) storeObject(ctx context.Context, obj Object, opts Options) (*webref.Ref, error) {
// 	data := obj.Marshal()
// 	return webref.Post(ctx, wfs.store, data, opts)
// }

// func (wfs *WebFS) getReadStore() webref.Read {
// 	// no options required for read
// 	return &Store{ms: wfs.store}
// }

// func (wfs *WebFS) getWriteStore(opts Options) webref.ReadWriteOnce {
// 	return &Store{ms: wfs.store, opts: opts}
// }

func dirpath(p string) string {
	x := path.Dir(p)
	if x == "." {
		x = ""
	}
	return ""
}

func basepath(p string) string {
	return path.Base(p)
}
