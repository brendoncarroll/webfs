package webfs

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"io"
	"log"
	"path"
	"sync"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/httpcell"
	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

const (
	RootVolumeID = "root"
)

type Ref = webref.Ref

type WebFS struct {
	root      *Volume
	volumes   sync.Map
	cells     sync.Map
	baseStore stores.ReadPost
}

func New(rootCell Cell, baseStore stores.ReadPost) (*WebFS, error) {
	wfs := &WebFS{
		volumes:   sync.Map{},
		cells:     sync.Map{},
		baseStore: baseStore,
	}
	wfs.cells.Store("", rootCell)
	wfs.addCell(rootCell)

	rv := &Volume{
		spec: &webfsim.VolumeSpec{
			Id: RootVolumeID,
		},
		cell: rootCell,
		baseObject: baseObject{
			fs:           wfs,
			parent:       nil,
			nameInParent: "",
		},
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
	if v, ok := o2.(*Volume); ok {
		return v, nil
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

func (wfs *WebFS) Remove(ctx context.Context, p string) error {
	p2 := parsePath(p)
	name := ""
	if len(p2) > 0 {
		name = p2[len(p2)-1]
	}

	parent, err := wfs.lookupParent(ctx, p2)
	if err != nil {
		return err
	}
	d, ok := parent.(*Dir)
	if !ok {
		return errors.New("cannot remove from non-dir parent")
	}
	return d.Delete(ctx, name)
}

func (wfs *WebFS) WalkObjects(ctx context.Context, f func(o Object) bool) error {
	v := wfs.root
	_, err := v.Walk(ctx, f)
	return err
}

func (wfs *WebFS) ParDo(ctx context.Context, f func(Object) bool) error {
	ch := make(chan Object, 16)

	const N = 16
	wg := sync.WaitGroup{}
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			for o := range ch {
				f(o)
			}
			wg.Done()
		}()
	}

	err := wfs.WalkObjects(ctx, func(o Object) bool {
		ch <- o
		return true
	})
	close(ch)

	wg.Wait()
	return err
}

func (wfs *WebFS) RefIter(ctx context.Context, f func(ref webref.Ref) bool) error {
	var topErr error
	err := wfs.ParDo(ctx, func(o Object) bool {
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

func (wfs *WebFS) NewVolume(ctx context.Context, p string, spec webfsim.VolumeSpec) (string, error) {
	if spec.Id == "" {
		spec.Id = generateVolId()
	}
	var cell cells.Cell
	switch x := spec.CellSpec.Spec.(type) {
	case *webfsim.CellSpec_Http:
		spec2 := httpcell.Spec{
			URL:     x.Http.Url,
			Headers: x.Http.Headers,
		}
		cell = httpcell.New(spec2)
	}
	if cell == nil {
		return "", errors.New("could not create cell")
	}
	_, err := cell.Get(ctx)
	if err != nil {
		return "", errors.New("could not access cell")
	}

	v := webfsim.Object{
		Value: &webfsim.Object_Volume{
			Volume: &spec,
		},
	}
	return spec.Id, wfs.putAt(ctx, parsePath(p), v)
}

func (wfs *WebFS) putAt(ctx context.Context, p Path, o webfsim.Object) error {
	parent, err := wfs.lookupParent(ctx, p)
	if err != nil {
		return err
	}
	name := ""
	if len(p) > 0 {
		last := len(p) - 1
		name = p[last]
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
	err = put(ctx, func(cur *webfsim.Object) (*webfsim.Object, error) {
		if cur != nil {
			return nil, errors.New("object already here")
		}
		return &o, nil
	})
	return err
}

func (wfs *WebFS) GetVolume(ctx context.Context, id string) (*Volume, error) {
	var vol *Volume
	err := wfs.WalkObjects(ctx, func(o Object) bool {
		v, ok := o.(*Volume)
		if ok && v.ID() == id {
			vol = v
			return false
		}
		return true
	})
	return vol, err
}

func (wfs *WebFS) ListVolumes(ctx context.Context) ([]*Volume, error) {
	vols := []*Volume{}
	err := wfs.ParDo(ctx, func(o Object) bool {
		vol, ok := o.(*Volume)
		if ok {
			vols = append(vols, vol)
		}
		return true
	})
	log.Println(err)
	return vols, nil
}

func (wfs *WebFS) addCell(cell Cell) {
	_, loaded := wfs.cells.LoadOrStore(cell.URL(), cell)
	if loaded {
		log.Println("loaded cell", cell)
	}
}

func (wfs *WebFS) getCellByURL(id string) Cell {
	cell, _ := wfs.cells.Load(id)
	if cell == nil {
		return nil
	}
	return cell.(Cell)
}

func (wfs *WebFS) setupCell(spec *webfsim.VolumeSpec, as *auxState) (Cell, error) {
	newCell, err := model2Cell(spec.CellSpec, as)
	if err != nil {
		return nil, err
	}

	cell, loaded := wfs.cells.LoadOrStore(newCell.URL(), newCell)
	if loaded {
		//log.Println("loaded new cell", newCell.URL())
	}
	if cell == nil {
		return nil, errors.New("could not create cell")
	}

	return cell.(cells.Cell), nil
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

func generateVolId() string {
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	buf := make([]byte, 16)
	rand.Reader.Read(buf)
	return encoder.EncodeToString(buf)
}
