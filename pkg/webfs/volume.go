package webfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/golang/protobuf/jsonpb"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

const VolumeCodec = webref.CodecJSON

var ErrCASFailed = errors.New("CAS failed. Should Retry")

type Cell = cells.Cell

type VolSpecMutator func(webfsim.VolumeSpec) webfsim.VolumeSpec
type VolumeMutator func(webfsim.Commit) (*webfsim.Commit, error)
type ObjectMutator func(*webfsim.Object) (*webfsim.Object, error)

func IdentityOM(x *webfsim.Object) (*webfsim.Object, error) {
	return x, nil
}

type Volume struct {
	spec  *webfsim.VolumeSpec
	cell  Cell
	opts  *Options
	store *Store

	baseObject
}

func NewVolume(ctx context.Context, parent Object, nameInParent string) (*Volume, error) {
	spec := &webfsim.VolumeSpec{
		Id: generateVolId(),
	}
	v, err := setupVolume(ctx, spec, parent.getFS(), parent, nameInParent)
	if err != nil {
		return nil, err
	}
	o := &webfsim.Object{
		Value: &webfsim.Object_Volume{v.spec},
	}
	if err := putAt(ctx, parent, nameInParent, o); err != nil {
		return nil, err
	}
	return v, nil
}

func setupVolume(ctx context.Context, spec *webfsim.VolumeSpec, fs *WebFS, parent Object, nameInParent string) (*Volume, error) {
	v := &Volume{
		spec: spec,
		baseObject: baseObject{
			fs:           fs,
			parent:       parent,
			nameInParent: nameInParent,
		},
		cell: cells.ZeroCell{},
	}

	if spec.CellSpec != nil {
		as := &auxState{v: v}
		cell, err := model2Cell(spec.CellSpec, as)
		if err != nil {
			return nil, err
		}
		v.cell = cell
	}

	return v, nil
}

func setupRootVolume(ctx context.Context, spec *webfsim.VolumeSpec, cell cells.Cell, fs *WebFS) (*Volume, error) {
	v := &Volume{
		spec: spec,
		baseObject: baseObject{
			fs: fs,
		},
		cell: cell,
	}
	return v, nil
}

func (v *Volume) ID() string {
	return v.spec.Id
}

func (v *Volume) GetAtPath(ctx context.Context, p Path, objs []Object, n int) ([]Object, error) {
	if len(p) == 0 {
		objs = append(objs, v)
	}
	if n >= 0 && len(objs) >= n {
		return objs, nil
	}
	o, err := v.getObject(ctx)
	if err != nil {
		return nil, err
	}
	if o != nil {
		objs, err = o.GetAtPath(ctx, p, objs, n)
		if err != nil {
			return nil, err
		}
	}
	return objs, nil
}

func (v *Volume) Lookup(ctx context.Context, p Path) (Object, error) {
	objs, err := v.GetAtPath(ctx, p, nil, -1)
	if err != nil {
		return nil, err
	}
	if len(objs) < 1 {
		return nil, nil
	}
	return objs[len(objs)-1], nil
}

func (v *Volume) Walk(ctx context.Context, f func(Object) bool) (bool, error) {
	cont := f(v)
	if !cont {
		return false, nil
	}

	cc, err := v.Get(ctx)
	if err != nil {
		return true, err
	}
	if cc == nil {
		return true, nil
	}
	if cc.ObjectRef == nil {
		return true, nil
	}
	mo := webfsim.Object{}
	s := v.getStore()
	if err := webref.GetAndDecode(ctx, s, *cc.ObjectRef, &mo); err != nil {
		return false, err
	}
	o, err := wrapObject(v, "", &mo)
	if err != nil {
		return false, err
	}

	return o.Walk(ctx, f)
}

func (v *Volume) SetOptions(ctx context.Context, opts *Options) error {
	return v.Apply(ctx, func(cx webfsim.Commit) (*webfsim.Commit, error) {
		yx := cx
		yx.Options = opts
		return &yx, nil
	})
}

func (v *Volume) PutCell(ctx context.Context, spec *webfsim.CellSpec) error {
	cell, err := model2Cell(spec, &auxState{v})
	if err != nil {
		return err
	}
	_, err = cell.Get(ctx)
	if err != nil {
		return err
	}
	err = v.ApplySpec(ctx, func(vspec webfsim.VolumeSpec) webfsim.VolumeSpec {
		y := vspec
		y.CellSpec = spec
		return y
	})
	if err != nil {
		return err
	}
	v.cell = cell
	log.Println(v)
	return nil
}

func (v *Volume) Get(ctx context.Context) (*webfsim.Commit, error) {
	return v.get(ctx)
}

func (v *Volume) get(ctx context.Context) (*webfsim.Commit, error) {
	if v.cell == nil {
		return nil, nil
	}
	data, err := v.cell.Get(ctx)
	if err != nil {
		return nil, err
	}
	if len(data) < 1 {
		return &webfsim.Commit{
			Options: DefaultOptions(),
		}, nil
	}

	m := webfsim.Commit{}
	if err := webref.Decode(webref.CodecJSON, data, &m); err != nil {
		return nil, err
	}
	v.opts = m.Options
	if v.opts == nil {
		v.opts = DefaultOptions()
	}

	return &m, nil
}

func (v *Volume) getObject(ctx context.Context) (Object, error) {
	m, err := v.get(ctx)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}
	if m.ObjectRef == nil {
		return nil, nil
	}
	mo := webfsim.Object{}
	s := v.getStore()
	if err := webref.GetAndDecode(ctx, s, *m.ObjectRef, &mo); err != nil {
		return nil, err
	}
	return wrapObject(v, "", &mo)
}

func (v *Volume) ApplyObject(ctx context.Context, fn ObjectMutator) error {
	opts := v.getOptions()
	ctx = webref.SetCodecCtx(ctx, opts.DataOpts.Codec)

	return v.Apply(ctx, func(cur webfsim.Commit) (*webfsim.Commit, error) {
		var o *webfsim.Object
		if cur.ObjectRef != nil {
			o = &webfsim.Object{}
			s := v.getStore()
			if err := webref.GetAndDecode(ctx, s, *cur.ObjectRef, o); err != nil {
				return nil, err
			}
		}

		o2, err := fn(o)
		if err != nil {
			return nil, err
		}

		var ref *webref.Ref
		if o2 != nil {
			ref, err = webref.EncodeAndPost(ctx, v.getStore(), o2)
			if err != nil {
				return nil, err
			}
		}

		next := webfsim.Commit{
			ObjectRef: ref,
			Options:   cur.Options,
		}
		return &next, nil
	})
}

func (v *Volume) Apply(ctx context.Context, f VolumeMutator) error {
	cell := v.cell

	const maxRetries = 10
	success := false
	for i := 0; !success && i < maxRetries; i++ {
		cur, err := cell.Get(ctx)
		if err != nil {
			return err
		}
		curV := webfsim.Commit{}
		if len(cur) > 0 {
			if err := webref.Decode(VolumeCodec, cur, &curV); err != nil {
				return err
			}
		} else {
			curV.Options = DefaultOptions()
		}

		nextV, err := f(curV)
		if err != nil {
			return err
		}
		if nextV == nil {
			panic("nil commit")
		}
		next, err := webref.Encode(VolumeCodec, nextV)
		if err != nil {
			return err
		}
		if bytes.Compare(cur, next) == 0 {
			return nil
		}

		success, err = cell.CAS(ctx, cur, next)
		if err != nil {
			return err
		}
	}

	if !success {
		return errors.New("could not complete CAS")
	}

	return nil
}

func (v *Volume) ApplySpec(ctx context.Context, f VolSpecMutator) error {
	if v.parent == nil {
		return errors.New("no spec for root cell")
	}

	var (
		spec *webfsim.VolumeSpec
	)

	mutator := func(x *webfsim.Object) (*webfsim.Object, error) {
		if x == nil {
			return nil, ErrConcurrentMod
		}
		x1, ok := x.Value.(*webfsim.Object_Volume)
		if !ok {
			return nil, ErrConcurrentMod
		}
		v := f(*x1.Volume)
		ret := &webfsim.Object{
			Value: &webfsim.Object_Volume{&v},
		}
		spec = &v
		return ret, nil
	}

	if err := apply(ctx, v.parent, v.nameInParent, mutator); err != nil {
		return err
	}

	newV, err := setupVolume(ctx, spec, v.fs, v.parent, v.nameInParent)
	if err != nil {
		return err
	}
	*v = *newV
	return nil
}

func (v *Volume) ApplyOptions(ctx context.Context, fn func(x *Options) *Options) error {
	return v.Apply(ctx, func(cx webfsim.Commit) (*webfsim.Commit, error) {
		yx := cx
		yx.Options = fn(cx.Options)
		return &yx, nil
	})
}

func (v *Volume) Path() Path {
	if v.parent == nil {
		return Path{}
	}
	p := v.parent.Path()
	if v.nameInParent != "" {
		p = append(p, v.nameInParent)
	}
	return p
}

func (v *Volume) RefIter(ctx context.Context, f func(webref.Ref) bool) (bool, error) {
	cc, err := v.Get(ctx)
	if err != nil {
		return false, err
	}
	cont := f(*cc.ObjectRef)
	return cont, nil
}

func (v *Volume) String() string {
	cellURL := ""
	if v.cell != nil {
		cellURL = v.cell.URL()
	}
	return fmt.Sprintf("Volume{%s}", cellURL)
}

func (v *Volume) Size() uint64 {
	return 0
}

func (v *Volume) Describe() string {
	m := jsonpb.Marshaler{
		Indent:       " ",
		EmitDefaults: true,
	}
	o := &webfsim.Object{
		Value: &webfsim.Object_Volume{v.spec},
	}
	buf := bytes.Buffer{}
	m.Marshal(&buf, o)
	return string(buf.Bytes())
}

func (v *Volume) URL() string {
	if v.cell == nil {
		return ""
	}
	return v.cell.URL()
}

func (v *Volume) Cell() cells.Cell {
	return v.cell
}

func (v *Volume) getStore() *Store {
	if v.store != nil {
		return v.store
	}

	opts := v.getOptions()
	store, err := BuildStore(opts.StoreSpecs, opts.DataOpts)
	if err != nil {
		panic("invalid store: " + err.Error())
	}
	if v.parent == nil {
		baseStore := v.fs.getStore()
		if baseStore != nil {
			store.router.AppendWith(baseStore.router)
		}
	} else {
		store.router.AppendWith(v.parent.getStore().router)
	}

	v.store = store
	return store
}

func (v *Volume) getOptions() *Options {
	switch {
	case v.parent == nil && v.opts != nil:
		return v.opts
	case v.parent == nil && v.opts == nil:
		return DefaultOptions()
	default:
		parentOpts := v.parent.getOptions()
		return MergeOptions(parentOpts, v.opts)
	}
}
