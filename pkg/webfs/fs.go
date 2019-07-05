package webfs

import (
	"context"
	"errors"
	"io"
	"log"
	"path"
	"strings"
	"sync"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Options = webref.Options

type WebFS struct {
	muxStore *webref.MuxStore
	cells    sync.Map
}

func New(rootCell Cell) (*WebFS, error) {
	wfs := &WebFS{
		muxStore: webref.NewMuxStore(),
		cells:    sync.Map{},
	}

	wfs.cells.Store("", rootCell)
	wfs.addCell(rootCell)

	return wfs, wfs.init()
}

func (wfs *WebFS) init() error {
	rootCell := wfs.getCell("")
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
	rootCell := wfs.getCell("")
	res, err := wfs.lookup(ctx, []Cell{rootCell}, "", nil, p)
	if err != nil {
		return nil, err
	}
	return res, err
}

func (wfs *WebFS) lookup(ctx context.Context, cellStack []Cell, relpath string, o *Object, p string) (*LookupResult, error) {
	if len(cellStack) > 1000 {
		return nil, errors.New("1000 cells deep is a little much")
	}

	// let this panic, there should always be a root cell
	cell := cellStack[len(cellStack)-1]
	cc, err := GetContents(ctx, cell)
	if err != nil {
		return nil, err
	}
	store := &Store{ms: wfs.muxStore, opts: cc.Options}

	o, err = wfs.loadObject(ctx, cc.ObjectRef)
	if err != nil {
		return nil, err
	}

	// if we have an empty path then we have finished the lookup
	if p == "" {
		cell := cellStack[len(cellStack)-1]
		res := LookupResult{
			Cell:    cell,
			RelPath: relpath,
			Object:  *o,
			Options: cc.Options,
		}
		return &res, nil
	}

	// if we don't have an object then do a load from the last cell in the stack
	if o == nil {
		return wfs.lookup(ctx, cellStack, relpath, o, p)
	}

	switch {
	// load the cell
	case o.Cell != nil:
		cell = cells.Make(*o.Cell)
		wfs.addCell(cell)
		cellStack = append(cellStack, cell)
		return wfs.lookup(ctx, cellStack, "", nil, p)
	// resolve the path
	case o.Dir != nil:
		parts := strings.Split(p, "/")
		if len(parts) < 2 {
			parts = append(parts, "")
		}
		o2, err := o.Dir.Get(ctx, store, parts[0])
		if o2 == nil {
			return nil, errors.New("no entry for " + p)
		}
		if err != nil {
			return nil, err
		}
		relpath := path.Join(relpath, parts[0])
		return wfs.lookup(ctx, cellStack, relpath, o2, parts[1])
	// errors below
	case o.File != nil:
		return nil, errors.New("cannot lookup path in file")
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
	store := &Store{ms: wfs.muxStore, opts: res.Options}
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
	store := &Store{ms: wfs.muxStore, opts: cc.Options}
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
	store := &Store{ms: wfs.muxStore, opts: opts}
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

func (wfs *WebFS) addCell(cell Cell) {
	_, loaded := wfs.cells.LoadOrStore(cell.ID(), cell)
	if loaded {
		log.Println("loaded cell", cell)
	}
}

func (wfs *WebFS) getCell(id string) Cell {
	cell, _ := wfs.cells.Load(id)
	if cell == nil {
		return nil
	}
	return cell.(Cell)
}

func (wfs *WebFS) loadObject(ctx context.Context, ref webref.Ref) (*Object, error) {
	obj := &Object{}
	data, err := ref.Deref(ctx, wfs.muxStore)
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
	return webref.Post(ctx, wfs.muxStore, data, opts)
}

func (wfs *WebFS) Cat(ctx context.Context, p string) (io.ReadCloser, error) {
	res, err := wfs.Lookup(ctx, p)
	if err != nil {
		return nil, err
	}
	store := &Store{ms: wfs.muxStore, opts: res.Options}
	o := res.Object
	switch {
	case o.File != nil:
		r := o.File.Reader(store)
		return r, nil
	default:
		panic("")
	}
}

func dirpath(p string) string {
	x := path.Dir(p)
	if x == "." {
		x = ""
	}
	return ""
}
