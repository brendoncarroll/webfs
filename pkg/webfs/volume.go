package webfs

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
)

var ErrCASFailed = errors.New("CAS failed. Should Retry")

type Cell = cells.Cell
type CASCell = cells.CASCell

type VolumeMutator func(models.Volume) (*models.Volume, error)
type ObjectMutator func(*models.Object) (*models.Object, error)

type Volume struct {
	cell Cell
	opts *Options

	baseObject
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
	data, err := v.getStore().Get(ctx, *cc.ObjectRef)
	if err != nil {
		return false, err
	}
	o, err := unmarshalObject(v, "", data)
	if err != nil {
		return false, err
	}

	return o.Walk(ctx, f)
}

func (v *Volume) Get(ctx context.Context) (*models.Volume, error) {
	data, err := v.cell.Load(ctx)
	if err != nil {
		return nil, err
	}
	if len(data) < 1 {
		return &models.Volume{
			Options: DefaultOptions(),
		}, nil
	}

	m := models.Volume{}
	if err := m.Unmarshal(data); err != nil {
		return nil, err
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
	data, err := v.getStore().Get(ctx, *m.ObjectRef)
	if err != nil {
		return nil, err
	}
	return unmarshalObject(v, "", data)
}

func (v *Volume) put(ctx context.Context, fn ObjectMutator) error {
	return v.apply(ctx, func(cur models.Volume) (*models.Volume, error) {
		var o *models.Object
		if cur.ObjectRef != nil {
			data, err := v.getStore().Get(ctx, *cur.ObjectRef)
			if err != nil {
				return nil, err
			}
			o = &models.Object{}
			if err := json.Unmarshal(data, o); err != nil {
				return nil, err
			}
		}

		o2, err := fn(o)
		if err != nil {
			return nil, err
		}

		data, _ := json.Marshal(o2)
		ref, err := v.getStore().Post(ctx, data)
		if err != nil {
			return nil, err
		}

		next := models.Volume{
			ObjectRef: ref,
			Options:   cur.Options,
		}
		return &next, nil
	})
}

func (v *Volume) apply(ctx context.Context, f VolumeMutator) error {
	wcell, ok := v.cell.(CASCell)
	if !ok {
		log.Println(v.cell)
		return errors.New("cell is not writeable")
	}

	const maxRetries = 10
	success := false
	for i := 0; !success && i < maxRetries; i++ {
		cur, err := wcell.Load(ctx)
		if err != nil {
			return err
		}
		curV := models.Volume{}
		if len(cur) > 0 {
			if err := curV.Unmarshal(cur); err != nil {
				return err
			}
		}

		nextV, err := f(curV)
		if err != nil {
			return err
		}
		next := nextV.Marshal()
		success, err = wcell.CAS(ctx, cur, next)
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
	return v.parent.Path()
}

func (v *Volume) RefIter(ctx context.Context, f func(Ref) bool) (bool, error) {
	cc, err := v.Get(ctx)
	if err != nil {
		return false, err
	}
	cont := f(*cc.ObjectRef)
	return cont, nil
}

func (v *Volume) String() string {
	return "Volume::" + v.cell.ID()
}

func (v *Volume) Size() uint64 {
	return 0
}

func (v *Volume) getStore() ReadWriteOnce {
	fs := v.getFS()
	return &Store{opts: v.getOptions(), ms: fs.store}
}

func (v *Volume) getOptions() Options {
	if v.opts != nil {
		return *v.opts
	}
	return DefaultOptions()
}
