package webfs

import (
	"context"
	"errors"
	"io"
	"log"
	"path"
	"sync"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/httpcell"
	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Ref = webref.Ref

type WebFS struct {
	root      *Volume
	cells     sync.Map
	baseStore stores.ReadWriteOnce
}

func New(rootCell Cell, baseStore stores.ReadWriteOnce) (*WebFS, error) {
	wfs := &WebFS{
		cells:     sync.Map{},
		baseStore: baseStore,
	}
	wfs.cells.Store("", rootCell)
	wfs.addCell(rootCell)

	rv := &Volume{
		cell: rootCell,
		baseObject: baseObject{
			fs:           wfs,
			parent:       nil,
			nameInParent: "",
		},
		opts: nil,
	}
	if err := rv.init(context.TODO()); err != nil {
		return nil, err
	}
	wfs.root = rv

	return wfs, nil
}

func (wfs *WebFS) Find(ctx context.Context, p string) ([]Object, error) {
	return wfs.find(ctx, parsePath(p))
}

func (wfs *WebFS) find(ctx context.Context, p Path) ([]Object, error) {
	return wfs.root.Find(ctx, p, nil)
}

func (wfs *WebFS) Lookup(ctx context.Context, p string) (Object, error) {
	p2 := parsePath(p)
	return wfs.lookup(ctx, p2)
}

func (wfs *WebFS) lookup(ctx context.Context, p Path) (Object, error) {
	o, err := wfs.root.Lookup(ctx, p)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errors.New("no entry at " + p.String())
	}
	return o, nil
}

func (wfs *WebFS) lookupParent(ctx context.Context, p Path) (Object, error) {
	parentPath := Path{}
	name := ""
	if len(p) > 0 {
		last := len(p) - 1
		name = p[last]
		parentPath = p[:last]
	}

	objs, err := wfs.find(ctx, parentPath)
	if err != nil {
		return nil, err
	}
	if len(objs) < 1 {
		return nil, nil
	}
	last := len(objs) - 1
	o := objs[last]

	dir, ok := o.(*Dir)
	if !ok {
		return o, nil
	}
	// check to see if there is a volume at that name
	o2, err := dir.Lookup(ctx, Path{name})
	if err != nil {
		return nil, err
	}
	if o2 == nil {
		return o, nil
	}
	return o2, nil
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
	p := parsePath(dst)
	name := ""
	if len(p) > 0 {
		name = p[len(p)-1]
	}

	parent, err := wfs.lookupParent(ctx, p)
	if err != nil {
		return err
	}
	f, err := newFile(ctx, parent, name)
	if err != nil {
		return err
	}
	if err := f.SetData(ctx, r); err != nil {
		return err
	}
	return nil
}

func (wfs *WebFS) Mkdir(ctx context.Context, p string) error {
	p2 := parsePath(p)
	name := ""
	if len(p2) > 0 {
		name = p2[len(p2)-1]
	}

	parent, err := wfs.lookupParent(ctx, p2)
	if err != nil {
		return err
	}
	_, err = newDir(ctx, parent, name)
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

func (wfs *WebFS) RefIter(ctx context.Context, f func(ref webref.Ref) bool) error {
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

func (wfs *WebFS) NewVolume(ctx context.Context, p string, spec models.VolumeSpec) error {
	cell := cells.Make(spec)
	if cell == nil {
		return errors.New("could not create cell")
	}
	_, err := cell.Get(ctx)
	if err != nil {
		return errors.New("could not access cell")
	}

	p2 := parsePath(p)
	parent, err := wfs.lookupParent(ctx, p2)
	if err != nil {
		return err
	}
	name := ""
	if len(p2) > 0 {
		last := len(p2) - 1
		name = p2[last]
	}

	var put func(context.Context, ObjectMutator) error
	switch x := parent.(type) {
	case *Volume:
		if name != "" {
			return errors.New("volumes do not support entries")
		}
		put = x.put
	case *Dir:
		put = func(ctx context.Context, fn ObjectMutator) error {
			return x.put(ctx, name, fn)
		}
	default:
		panic("lookup returned file")
	}

	err = put(ctx, func(cur *models.Object) (*models.Object, error) {
		return &models.Object{
			Value: &models.Object_Volume{
				Volume: &spec,
			},
		}, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (wfs *WebFS) ListVolumes(ctx context.Context) ([]*Volume, error) {
	vols := []*Volume{}
	err := wfs.WalkObjects(ctx, func(o Object) bool {
		vol, ok := o.(*Volume)
		if ok {
			vols = append(vols, vol)
		}
		return true
	})
	return vols, err
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

func (wfs *WebFS) getCellBySpec(spec *models.VolumeSpec) (Cell, error) {
	var newCell cells.Cell

	switch x := spec.CellSpec.(type) {
	case *models.VolumeSpec_Httpcell:
		spec2 := httpcell.Spec{
			URL:        x.Httpcell.Url,
			AuthHeader: x.Httpcell.AuthHeader,
		}
		newCell = httpcell.New(spec2)
	default:
		return nil, errors.New("cell type not recognized")
	}

	cell, loaded := wfs.cells.LoadOrStore(newCell.ID(), newCell)
	if loaded {
		log.Println("loaded new cell", newCell.ID())
	}
	if cell == nil {
		return nil, errors.New("could not create cell")
	}
	return cell.(Cell), nil
}

func (wfs *WebFS) getStore() *Store {
	routes := []stores.StoreRoute{
		{
			Prefix: "",
			Store:  wfs.baseStore,
		},
	}
	return &Store{router: stores.NewRouter(routes)}
}

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
