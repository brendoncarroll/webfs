package webref

import (
	"context"
	fmt "fmt"
	"log"
	mrand "math/rand"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

func PostMirror(ctx context.Context, s stores.Post, o Options, data []byte) (*Ref, error) {
	refs := []*Ref{}
	for k, v := range o.Replicas {
		for i := 0; i < int(v); i++ {
			prefix := k
			cref, err := PostCrypto(ctx, s, o, prefix, data)
			if err != nil {
				return nil, err
			}
			refs = append(refs, cref)
		}
	}
	return &Ref{
		Ref: &Ref_Mirror{
			&Mirror{
				Replicas: refs,
			},
		},
	}, nil
}

func GetMirror(ctx context.Context, s stores.Read, m *Mirror) ([]byte, error) {
	l := len(m.Replicas)
	count := 0
	i := mrand.Int() % l
	errs := []error{}
	for count < l {
		ref := m.Replicas[i]
		data, err := Get(ctx, s, *ref)
		if err == nil {
			return data, nil
		}
		errs = append(errs, err)
		log.Println(err)

		i = (i + 1) % l
		count++
	}
	return nil, fmt.Errorf("Errors from all replicas")
}

func DeleteMirror(ctx context.Context, s stores.Delete, r *Mirror) error {
	for _, x := range r.Replicas {
		if err := Delete(ctx, s, *x); err != nil {
			return err
		}
	}
	return nil
}

func CheckMirror(ctx context.Context, s stores.Check, r *Mirror) ([]RefStatus, error) {
	stats := []RefStatus{}
	for _, x := range r.Replicas {
		stats2 := Check(ctx, s, *x)
		stats = append(stats, stats2...)
	}
	return stats, nil
}
