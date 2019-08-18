package webref

import (
	"context"
	"errors"
	"fmt"
	"log"
	mrand "math/rand"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

func PostRepPack(ctx context.Context, store stores.WriteOnce, o Options, data []byte) (*RepPackRef, error) {
	refs := []*RepPackRef{}
	if len(o.Replicas) == 0 {
		return nil, fmt.Errorf("no prefixes to post to")
	}
	for k, v := range o.Replicas {
		for i := 0; i < int(v); i++ {
			prefix := k
			cref, err := PostCrypto(ctx, store, prefix, data, o)
			if err != nil {
				return nil, err
			}
			ref := &RepPackRef{
				Ref: &RepPackRef_Single{Single: cref},
			}
			refs = append(refs, ref)
		}
	}

	if len(refs) > 1 {
		return &RepPackRef{
			Ref: &RepPackRef_Mirror{
				Mirror: &Mirror{
					Replicas: refs,
				},
			},
		}, nil
	}

	ref := refs[0]
	return ref, nil
}

func GetRepPack(ctx context.Context, s stores.Read, r *RepPackRef) ([]byte, error) {
	switch x := r.Ref.(type) {
	case *RepPackRef_Single:
		return GetCrypto(ctx, s, x.Single)
	case *RepPackRef_Mirror:
		return GetMirror(ctx, s, x.Mirror)
	case *RepPackRef_Slice:
		panic("not implemented")
	default:
		return nil, errors.New("invalid ref")
	}
}

func (r *RepPackRef) GetURLs() []string {
	switch x := r.Ref.(type) {
	case *RepPackRef_Single:
		return []string{x.Single.Url}
	case *RepPackRef_Mirror:
		urls := []string{}
		for _, ref := range x.Mirror.Replicas {
			urls = append(urls, ref.GetURLs()...)
		}
		return urls
	case *RepPackRef_Slice:
		return []string{x.Slice.Ref.Url}
	default:
		return nil
	}
}

func GetMirror(ctx context.Context, s stores.Read, m *Mirror) ([]byte, error) {
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
