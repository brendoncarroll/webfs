package webref

import (
	"context"
	"errors"
	"fmt"
	"log"
	mrand "math/rand"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

type RepPackRef struct {
	Single *CryptoRef `json:"single,omitempty"`
	Mirror *Mirror    `json:"mirror,omitempty"`
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
		return &RepPackRef{
			Mirror: &Mirror{
				Replicas: refs,
			},
		}, nil
	}

	ref := refs[0]
	return &ref, nil
}

func GetRepPack(ctx context.Context, s stores.Read, r RepPackRef) ([]byte, error) {
	switch {
	case r.Single != nil:
		return GetCrypto(ctx, s, *r.Single)
	case r.Mirror != nil:
		return GetMirror(ctx, s, *r.Mirror)
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

func GetMirror(ctx context.Context, s stores.Read, m Mirror) ([]byte, error) {
	l := len(m.Replicas)
	count := 0
	i := mrand.Int() % l
	errs := []error{}
	for count < l {
		ref := m.Replicas[i]
		data, err := GetRepPack(ctx, s, ref)
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
