package l2

import (
	"context"
	"errors"

	"github.com/brendoncarroll/webfs/pkg/l0"
	"github.com/brendoncarroll/webfs/pkg/l1"
)

type Options struct {
	Replicas map[string]int
}

type Ref struct {
	Single *l1.Ref `json:"single"`
	Mirror *Mirror `json:"mirror"`
}

func Post(ctx context.Context, store l0.WriteOnce, data []byte, o Options) (*Ref, error) {
	refs := []Ref{}
	for k, v := range o.Replicas {
		for i := 0; i < v; i++ {
			prefix := k
			l1opts := l1.DefaultOptions()
			l1ref, err := l1.Post(ctx, store, prefix, data, l1opts)
			if err != nil {
				return nil, err
			}
			ref := Ref{
				Single: l1ref,
			}
			refs = append(refs, ref)
		}
	}
	if len(refs) > 1 {
		// mirror
		panic("not done")
	}

	ref := refs[0]
	return &ref, nil
}

func (r *Ref) Deref(ctx context.Context, s l0.Read) ([]byte, error) {
	switch {
	case r.Single != nil:
		return r.Single.Deref(ctx, s)
	case r.Mirror != nil:
		return r.Mirror.Deref(ctx, s)
	default:
		return nil, errors.New("invalid ref")
	}
}

type Mirror struct {
	Replicas []l1.Ref `json:"replicas"`
}

// func MirrorRef(ctx context.Context, s l1.WriteOnce, data []byte) (*MirrorRef, error) {
// 	totalReplicas := 0
// 	for _, n := range ws.Replicas {
// 		totalReplicas += n
// 	}
// 	replicas := make([]*Ref, n)
// 	return nil, &MirrorRef{
// 		replicas: replicas,
// 	}
// }

func (m *Mirror) Deref(ctx context.Context, s l0.Read) ([]byte, error) {
	// 	l := len(m.Replicas)
	// 	count := 0
	// 	i := rand.Int() % l
	// 	errs := []error{}
	// 	for count < l {
	// 		ref := m.Replicas[i]
	// 		data, err := ref.Deref(ctx, context.Context)
	// 		if err == nil {
	// 			return data, nil
	// 		}
	// 		errs = append(errs, err)
	// 		log.Println(err)

	// 		i = (i + 1) % l
	// 		count++
	// 	}
	// 	return nil, fmt.Errorf("Errors from all replicas")
	return nil, nil
}
