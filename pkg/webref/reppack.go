package webref

import (
	"context"
	"errors"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

type RepPackRef struct {
	Single *CryptoRef `json:"single"`
	Mirror *Mirror    `json:"mirror"`
}

func PostRepPack(ctx context.Context, store stores.WriteOnce, data []byte, o Options) (*RepPackRef, error) {
	refs := []RepPackRef{}
	for k, v := range o.Replicas {
		for i := 0; i < v; i++ {
			prefix := k
			l1ref, err := PostCrypto(ctx, store, prefix, data, o)
			if err != nil {
				return nil, err
			}
			ref := RepPackRef{
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

func (r *RepPackRef) Deref(ctx context.Context, s stores.Read) ([]byte, error) {
	switch {
	case r.Single != nil:
		return r.Single.Deref(ctx, s)
	case r.Mirror != nil:
		return r.Mirror.Deref(ctx, s)
	default:
		return nil, errors.New("invalid ref")
	}
}

func (r *Ref) GetURLs() []string {
	switch {
	case r.Single != nil:
		return []string{r.Single.URL}
	default:
		return nil
	}
}

type Mirror struct {
	Replicas []RepPackRef `json:"replicas"`
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

func (m *Mirror) Deref(ctx context.Context, s stores.Read) ([]byte, error) {
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
