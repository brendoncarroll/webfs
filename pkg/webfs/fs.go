package webfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"path"
	"strings"
	"sync"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Options = webref.Options

type WebFS struct {
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
	return wfs, wfs.init()
}

func (wfs *WebFS) init() error {
	rootCell := wfs.getCellByID("")
	ctx := context.TODO()
	data, err := rootCell.Load(ctx)
	if err != nil {
		return err
	}
	// if there is nothing in the root load an empty dir
	if len(data) == 0 {
		o := Object{
			Dir: NewDir(),
		}
		opts := webref.DefaultOptions()
		ref, err := wfs.storeObject(ctx, o, opts)
		if err != nil {
			return err
		}
		cc := CellContents{
			Options:   opts,
			ObjectRef: *ref,
		}
		accepted, err := rootCell.(CASCell).CAS(ctx, nil, cc.Marshal())
		if err != nil {
			return err
		}
		if !accepted {
			return errors.New("could not initialize root cell")
		}
	}
	return nil
}

type LookupResult struct {
	Cell    Cell
	RelPath string
	Object  Object
	Options Options
}

func (wfs *WebFS) Lookup(ctx context.Context, p string) (*LookupResult, error) {
	if len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	rootCell := wfs.getCellByID("")
	res, err := wfs.lookupInCell(ctx, rootCell, p)
	if err != nil {
		return nil, err
	}
	return res, err
}

func (wfs *WebFS) lookupInCell(ctx context.Context, cell Cell, p string) (*LookupResult, error) {
	cc, err := GetContents(ctx, cell)
	if err != nil {
		return nil, err
	}
	o, err := wfs.loadObject(ctx, cc.ObjectRef)
	if err != nil {
		return nil, err
	}
	res, err := wfs.lookupInObject(ctx, o, p)
	if err != nil {
		return nil, err
	}
	if res.Cell == nil {
		res.Cell = cell
		res.RelPath = p
		res.Options = cc.Options
	}

	return res, nil
}

func (wfs *WebFS) lookupInObject(ctx context.Context, o *Object, p string) (*LookupResult, error) {
	if p == "" {
		res := &LookupResult{
			Object: *o,
		}
		return res, nil
	}

	switch {
	case o.Cell != nil:
		cell := wfs.getCellBySpec(*o.Cell)
		return wfs.lookupInCell(ctx, cell, p)

	case o.Dir != nil:
		parts := strings.Split(p, "/")
		if len(parts) < 2 {
			parts = append(parts, "")
		}
		store := &Store{ms: wfs.store}
		o2, err := o.Dir.Get(ctx, store, parts[0])
		if err != nil {
			return nil, err
		}
		if o2 == nil {
			return nil, errors.New("no entry for " + p)
		}
		res := &LookupResult{
			Object: *o2,
		}
		return res, nil

	case o.File != nil:
		return nil, errors.New("cannot lookup in file")

	default:
		return nil, errors.New("empty object")

	}
}

func (wfs *WebFS) Ls(ctx context.Context, p string) ([]DirEntry, error) {
	res, err := wfs.Lookup(ctx, p)
	if err != nil {
		return nil, err
	}
	o := res.Object
	if o.Dir == nil {
		return nil, errors.New("Cannot ls non dir")
	}
	store := &Store{ms: wfs.store, opts: res.Options}
	entries, err := o.Dir.Entries(ctx, store)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (wfs *WebFS) ImportFile(ctx context.Context, r io.Reader, dst string) error {
	parts := strings.Split(dst, "/")
	if len(parts) < 2 {
		parts = []string{"", parts[0]}
	}
	res, err := wfs.Lookup(ctx, parts[0])
	if err != nil {
		return err
	}
	// path from the cell
	relPath := path.Join(res.RelPath, parts[1])

	cc, err := GetContents(ctx, res.Cell)
	if err != nil {
		return err
	}
	store := &Store{ms: wfs.store, opts: cc.Options}
	f, err := NewFile(ctx, store, r)
	if err != nil {
		return err
	}
	fo := Object{File: f}

	return Apply(ctx, res.Cell, func(cur CellContents) (*CellContents, error) {
		curO, err := wfs.loadObject(ctx, cur.ObjectRef)
		if err != nil {
			return nil, err
		}

		o, err := wfs.PutObject(ctx, *curO, fo, relPath, cur.Options)
		if err != nil {
			return nil, err
		}

		oRef, err := wfs.storeObject(ctx, *o, cc.Options)
		if err != nil {
			return nil, err
		}
		return &CellContents{
			Options:   cur.Options,
			ObjectRef: *oRef,
		}, nil
	})
}

// PutObject - maybe ReplaceInParent is a better name
// you will get back an updated version of parent, with target inserted at relPath
// under the parent
func (wfs *WebFS) PutObject(ctx context.Context, parent, target Object, relPath string, opts Options) (*Object, error) {
	store := &Store{ms: wfs.store, opts: opts}
	log.Println("PutObject", relPath)
	switch {
	case parent.Cell != nil:
		// maybe there is something we can do about this, but it shouldn't really happen.
		panic("unmounted cell")

	case parent.File != nil && relPath == "":
		// replace the file with the new object
		return &target, nil
	case parent.Dir != nil && relPath == "":
		return &target, nil

	case parent.Dir != nil:
		parts := strings.SplitN(relPath, "/", 2)

		var newChild *Object
		// need to recurse
		if len(parts) == 2 {
			child, err := parent.Dir.Get(ctx, store, parts[0])
			if err != nil {
				return nil, err
			}
			if child == nil {
				return nil, errors.New("no such path")
			}
			child, err = wfs.PutObject(ctx, *child, target, parts[1], opts)
			if err != nil {
				return nil, err
			}
			newChild = child
		}
		if len(parts) == 1 {
			newChild = &target
		}

		dirEnt := DirEntry{
			Name:   parts[0],
			Object: *newChild,
		}
		do, err := parent.Dir.Put(ctx, store, dirEnt)
		if err != nil {
			return nil, err
		}
		o := Object{Dir: do}
		return &o, nil
	default:
		return nil, errors.New("must have dir to put into")
	}
}

func (wfs *WebFS) Mkdir(ctx context.Context, p string) error {
	dirp := path.Dir(p)
	if dirp == "." {
		dirp = ""
	}
	basep := path.Base(p)
	res, err := wfs.Lookup(ctx, dirp)
	if err != nil {
		return err
	}
	o := res.Object
	switch {
	case o.Dir != nil:
	case o.Cell != nil:
	default:
		return errors.New("Can't create dir in non dir")
	}

	return Apply(ctx, res.Cell, func(cur CellContents) (*CellContents, error) {
		d := NewDir()
		do := Object{Dir: d}
		curO, err := wfs.loadObject(ctx, cur.ObjectRef)
		if err != nil {
			return nil, err
		}
		nextO, err := wfs.PutObject(ctx, *curO, do, basep, cur.Options)
		if err != nil {
			return nil, err
		}
		ref, err := wfs.storeObject(ctx, *nextO, cur.Options)
		if err != nil {
			return nil, err
		}
		return &CellContents{
			Options:   cur.Options,
			ObjectRef: *ref,
		}, nil
	})
}

func (wfs *WebFS) Cat(ctx context.Context, p string) (io.ReadCloser, error) {
	res, err := wfs.Lookup(ctx, p)
	if err != nil {
		return nil, err
	}
	store := &Store{ms: wfs.store, opts: res.Options}
	o := res.Object
	switch {
	case o.File != nil:
		r := o.File.Reader(store)
		return r, nil
	default:
		return nil, fmt.Errorf("can't cat non-file %v", o)
	}
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

func (wfs *WebFS) getCellBySpec(spec CellSpec) Cell {
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

func (wfs *WebFS) loadObject(ctx context.Context, ref webref.Ref) (*Object, error) {
	obj := &Object{}
	data, err := ref.Deref(ctx, wfs.store)
	if err != nil {
		return nil, err
	}
	if err := obj.Unmarshal(data); err != nil {
		return nil, err
	}
	return obj, nil
}

func (wfs *WebFS) storeObject(ctx context.Context, obj Object, opts Options) (*webref.Ref, error) {
	data := obj.Marshal()
	return webref.Post(ctx, wfs.store, data, opts)
}

func dirpath(p string) string {
	x := path.Dir(p)
	if x == "." {
		x = ""
	}
	return ""
}
