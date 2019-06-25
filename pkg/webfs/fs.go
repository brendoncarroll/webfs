package webfs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"path"
	"strings"
	"sync"
)

type Options struct {
	Replicas map[string]int
}

type WebFS struct {
	sb    *Superblock
	store *Store

	mu    sync.Mutex
	cells map[string]Cell
	//mounts map[string]CellMount
}

func New(sb *Superblock) (*WebFS, error) {
	wfs := &WebFS{
		sb:    sb,
		store: NewStore(),
		cells: map[string]Cell{},
	}

	rootCell := &RootCell{superblock: sb}
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
		objData := o.Marshal()
		accepted, err := rootCell.(CASCell).CAS(ctx, nil, objData)
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

func (wfs *WebFS) lookup(ctx context.Context, cells []Cell, relpath string, o *Object, p string) (*LookupResult, error) {
	if len(cells) > 1000 {
		return nil, errors.New("1000 cells deep is a little much")
	}

	// If we don't have an object then do a load from the last cell in the stack
	if o == nil {
		// let this panic, there should always be a root cell
		cell := cells[len(cells)-1]
		o, err := objectFromCell(ctx, cell)
		if err != nil {
			return nil, err
		}
		return wfs.lookup(ctx, cells, relpath, o, p)
	}

	// if we have an empty path then we have finished the lookup
	if p == "" {
		cell := cells[len(cells)-1]
		res := LookupResult{
			Cell:    cell,
			RelPath: relpath,
			Object:  *o,
		}
		return &res, nil
	}

	switch {
	// load the cell
	case o.Cell != nil:
		cell := wfs.getCell(o.Cell.ID())
		if cell == nil {
			cell = NewCell(*o.Cell)
			wfs.addCell(cell)
		}
		cells = append(cells, cell)
		return wfs.lookup(ctx, cells, "", nil, p)
	// resolve the path
	case o.Dir != nil:
		parts := strings.Split(p, "/")
		if len(parts) < 2 {
			parts = append(parts, "")
		}
		o2, err := o.Dir.Get(ctx, wfs.store, parts[0])
		if o2 == nil {
			return nil, errors.New("no entry for " + p)
		}
		if err != nil {
			return nil, err
		}
		relpath := path.Join(relpath, parts[0])
		return wfs.lookup(ctx, cells, relpath, o2, parts[1])
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
	entries, err := o.Dir.Entries(ctx, wfs.store)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (wfs *WebFS) ImportFile(ctx context.Context, r io.Reader, dst string) error {
	f, err := NewFile(ctx, wfs.store, r)
	if err != nil {
		return err
	}
	fo := Object{File: f}

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
	return Apply(ctx, res.Cell, func(cur Object) (*Object, error) {
		return wfs.PutObject(ctx, cur, fo, relPath)
	})
}

// PutObject - maybe ReplaceInParent is a better name
// you will get back an updated version of parent, with target inserted at relPath
// under the parent
func (wfs *WebFS) PutObject(ctx context.Context, parent, target Object, relPath string) (*Object, error) {
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
			child, err := parent.Dir.Get(ctx, wfs.store, parts[0])
			if err != nil {
				return nil, err
			}
			if child == nil {
				return nil, errors.New("no such path")
			}
			child, err = wfs.PutObject(ctx, *child, target, parts[1])
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
		do, err := parent.Dir.Put(ctx, wfs.store, dirEnt)
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
	return Apply(ctx, res.Cell, func(cur Object) (*Object, error) {
		d := NewDir()
		do := Object{Dir: d}
		return wfs.PutObject(ctx, cur, do, basep)
	})
}

func (wfs *WebFS) addCell(cell Cell) {
	wfs.mu.Lock()
	defer wfs.mu.Unlock()

	if _, exist := wfs.cells[cell.ID()]; !exist {
		wfs.cells[cell.ID()] = cell
	}
}

func (wfs *WebFS) getCell(id string) Cell {
	wfs.mu.Lock()
	defer wfs.mu.Unlock()
	cell, exists := wfs.cells[id]
	if !exists {
		return nil
	}
	return cell
}

func objectFromCell(ctx context.Context, cell Cell) (*Object, error) {
	data, err := cell.Load(ctx)
	if err != nil {
		return nil, err
	}
	o := Object{}
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

func (wfs *WebFS) Cat(ctx context.Context, p string) (io.ReadCloser, error) {
	res, err := wfs.Lookup(ctx, p)
	if err != nil {
		return nil, err
	}
	o := res.Object
	switch {
	case o.File != nil:
		r := o.File.Reader(wfs.store)
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
