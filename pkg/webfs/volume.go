package webfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/golang/protobuf/jsonpb"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

const VolumeCodec = webref.CodecJSON

var ErrCASFailed = errors.New("CAS failed. Should Retry")

type Cell = cells.Cell

type VolumeMutator func(webfsim.Commit) (*webfsim.Commit, error)
type ObjectMutator func(*webfsim.Object) (*webfsim.Object, error)

type Volume struct {
	spec  *webfsim.VolumeSpec
	cell  Cell
	opts  *Options
	store *Store

	baseObject
}

func (v *Volume) ID() string {
	return v.spec.Id
}

func (v *Volume) Find(ctx context.Context, p Path, objs []Object) ([]Object, error) {
	if len(p) == 0 {
		objs = append(objs, v)
	}

	o, err := v.getObject(ctx)
	if err != nil {
		return nil, err
	}
	if o != nil {
		objs, err = o.Find(ctx, p, objs)
		if err != nil {
			return nil, err
		}
	}
	return objs, nil
}

func (v *Volume) Lookup(ctx context.Context, p Path) (Object, error) {
	o, err := v.getObject(ctx)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, nil
	}
	return o.Lookup(ctx, p)
}

func (v *Volume) Walk(ctx context.Context, f func(Object) bool) (bool, error) {
	cont := f(v)
	if !cont {
		return false, nil
	}

	cc, err := v.Get(ctx)
	if err != nil {
		return false, err
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

func (v *Volume) ChangeOptions(ctx context.Context, fn func(x *Options) *Options) error {
	return v.Apply(ctx, func(cx webfsim.Commit) (*webfsim.Commit, error) {
		yx := cx
		yx.Options = fn(cx.Options)
		return &yx, nil
	})
}

func (v *Volume) SetOptions(ctx context.Context, opts *Options) error {
	return v.Apply(ctx, func(cx webfsim.Commit) (*webfsim.Commit, error) {
		yx := cx
		yx.Options = opts
		return &yx, nil
	})
}

func (v *Volume) Get(ctx context.Context) (*webfsim.Commit, error) {
	return v.get(ctx)
}

func (v *Volume) get(ctx context.Context) (*webfsim.Commit, error) {
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
	m, err := v.Get(ctx)
	if err != nil {
		return nil, err
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

func (v *Volume) put(ctx context.Context, fn ObjectMutator) error {
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

		ref, err := webref.EncodeAndPost(ctx, v.getStore(), o2)
		if err != nil {
			return nil, err
		}

		next := webfsim.Commit{
			ObjectRef: ref,
			Options:   cur.Options,
		}
		return &next, nil
	})
}

func (v *Volume) Apply(ctx context.Context, f VolumeMutator) error {
	const maxRetries = 10
	success := false
	for i := 0; !success && i < maxRetries; i++ {
		cur, err := v.cell.Get(ctx)
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
		next, err := webref.Encode(VolumeCodec, nextV)
		if err != nil {
			return err
		}
		success, err = v.cell.CAS(ctx, cur, next)
		if err != nil {
			return err
		}
	}

	if !success {
		return errors.New("could not complete CAS")
	}

	return nil
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
	return fmt.Sprintf("Volume{%s}", v.cell.URL())
}

func (v *Volume) Size() uint64 {
	return 0
}

func (v *Volume) Describe() string {
	m := jsonpb.Marshaler{
		Indent:       " ",
		EmitDefaults: true,
	}
	buf := bytes.Buffer{}
	m.Marshal(&buf, v.spec)
	return string(buf.Bytes())
}

func (v *Volume) URL() string {
	return v.cell.URL()
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
	case v.parent == nil:
		return v.opts
	default:
		parentOpts := v.parent.getOptions()
		return MergeOptions(parentOpts, v.opts)
	}
}

func (v *Volume) init(ctx context.Context) error {
	err := v.Apply(ctx, func(x webfsim.Commit) (*webfsim.Commit, error) {
		y := x
		if y.Options == nil {
			y.Options = DefaultOptions()
		}
		return &y, nil
	})
	if err != nil {
		return err
	}
	if _, err := v.get(ctx); err != nil {
		return err
	}
	return err
}
