package webfs

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"io"
	"log"
	"os"
	"path"
	"sync"

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
	baseStore stores.ReadPost
}

func New(rootCell Cell, baseStore stores.ReadPost) (*WebFS, error) {
	wfs := &WebFS{
		baseStore: baseStore,
	}

	v, err := setupRootVolume(
		context.TODO(),
		&webfsim.VolumeSpec{
			Id: RootVolumeID,
		},
		rootCell,
		wfs,
	)
	if err != nil {
		return nil, err
	}
	wfs.root = v

	return wfs, nil
}

func (wfs *WebFS) GetAtPath(ctx context.Context, p string) ([]Object, error) {
	return wfs.getAtPath(ctx, ParsePath(p), -1)
}

func (wfs *WebFS) getAtPath(ctx context.Context, p Path, max int) ([]Object, error) {
	return wfs.root.GetAtPath(ctx, p, nil, max)
}

func (wfs *WebFS) Lookup(ctx context.Context, p string) (Object, error) {
	return wfs.lookup(ctx, ParsePath(p))
}

func (wfs *WebFS) lookup(ctx context.Context, p Path) (Object, error) {
	objs, err := wfs.getAtPath(ctx, p, -1)
	if err != nil {
		return nil, err
	}
	if len(objs) < 1 {
		return nil, os.ErrNotExist
	}
	return objs[len(objs)-1], nil
}

func (wfs *WebFS) lookupParent(ctx context.Context, p Path) (Object, string, error) {
	parentPath := Path{}
	name := ""
	if len(p) > 0 {
		last := len(p) - 1
		name = p[last]
		parentPath = p[:last]
	}
	objs, err := wfs.getAtPath(ctx, parentPath, -1)
	if err != nil {
		return nil, "", err
	}
	if len(objs) < 1 {
		return nil, "", nil
	}
	last := len(objs) - 1
	o := objs[last]
	dir, ok := o.(*Dir)
	if !ok {
		return o, name, nil
	}

	// check to see if there is a volume at that name
	o2, err := dir.Get(ctx, name)
	if err != nil {
		return nil, "", err
	}
	if o2 == nil {
		return o, name, nil
	}
	if v, ok := o2.(*Volume); ok {
		return v, "", nil
	}
	return o, name, nil
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
	parent, name, err := wfs.lookupParent(ctx, ParsePath(dst))
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
	parent, name, err := wfs.lookupParent(ctx, ParsePath(p))
	if err != nil {
		return err
	}
	_, err = NewDir(ctx, parent, name)
	if err != nil {
		return err
	}
	return err
}

func (wfs *WebFS) Touch(ctx context.Context, p string) (*File, error) {
	parent, name, err := wfs.lookupParent(ctx, ParsePath(p))
	if err != nil {
		return nil, err
	}
	f, err := newFile(ctx, parent, name)
	if err != nil {
		return nil, err
	}
	return f, nil
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
	parent, name, err := wfs.lookupParent(ctx, ParsePath(p))
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

func (wfs *WebFS) NewVolume(ctx context.Context, p string) (*Volume, error) {
	parent, name, err := wfs.lookupParent(ctx, ParsePath(p))
	if err != nil {
		return nil, err
	}

	v, err := NewVolume(ctx, parent, name)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (wfs *WebFS) PutAt(ctx context.Context, p string, index int, o *webfsim.Object) error {
	var (
		parent Object
		name   string
		err    error
	)
	if index-1 < 0 {
		parent, name, err = wfs.lookupParent(ctx, ParsePath(p))
		if err != nil {
			return err
		}
	} else {
		objs, err := wfs.GetAtPath(ctx, p)
		if err != nil {
			return err
		}
		if index-1 >= len(objs) {
			return errors.New("no parent object found")
		}
		parent = objs[index-1]
	}

	return putAt(ctx, parent, name, o)
}

func (wfs *WebFS) putAt(ctx context.Context, p Path, o webfsim.Object) error {
	parent, name, err := wfs.lookupParent(ctx, p)
	if err != nil {
		return err
	}

	var put func(context.Context, ObjectMutator) error
	switch x := parent.(type) {
	case *Volume:
		if name != "" {
			return errors.New("volumes do not support entries")
		}
		put = x.ApplyObject
	case *Dir:
		put = func(ctx context.Context, fn ObjectMutator) error {
			return x.ApplyEntry(ctx, name, fn)
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
	if vol == nil {
		return nil, ErrNotExist
	}
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
	if err != nil {
		log.Println(err)
	}
	return vols, nil
}

func (wfs *WebFS) Move(ctx context.Context, o Object, dst string) error {
	if _, err := wfs.Copy(ctx, o, dst); err != nil {
		return err
	}
	return wfs.Remove(ctx, o.Path().String())
}

func (wfs *WebFS) Copy(ctx context.Context, o Object, dst string) (Object, error) {
	parent, name, err := wfs.lookupParent(ctx, ParsePath(dst))
	if err != nil {
		return nil, err
	}

	parV := containingVolume(parent)
	oV := containingVolume(o)

	if parV.spec.Id != oV.spec.Id {
		return nil, errors.New("copy accross volumes not yet supported")
	}

	switch o := o.(type) {
	case *File:
		oCopy := *o
		oCopy.parent = parent
		oCopy.nameInParent = name
		return &oCopy, oCopy.Sync(ctx)
	case *Dir:
		oCopy := *o
		oCopy.parent = parent
		oCopy.nameInParent = name
		return &oCopy, oCopy.Sync(ctx)
	default:
		return nil, ErrObjectType
	}
}

func (wfs *WebFS) DeleteAt(ctx context.Context, p string, index int) error {
	objs, err := wfs.getAtPath(ctx, ParsePath(p), index+1)
	if err != nil {
		return err
	}
	if len(objs) <= index {
		return ErrNotExist
	}
	o := objs[index]

	return deleteAt(ctx, o.getParent(), o.getName())
}

func (wfs *WebFS) getStore() *Store {
	if wfs.baseStore == nil {
		return nil
	}
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
