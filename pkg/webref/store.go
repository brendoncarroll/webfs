package webref

import (
	"context"
	"errors"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

type Store interface {
	Getter
	Poster
	Deleter
}

type BasicStore struct {
	Store stores.ReadPost
}

func (s *BasicStore) Get(ctx context.Context, ref *Ref) ([]byte, error) {
	return Get(ctx, s.Store, *ref)
}

func (s *BasicStore) Check(ctx context.Context, ref *Ref) []RefStatus {
	return nil
}

func (s *BasicStore) Post(ctx context.Context, data []byte) (*Ref, error) {
	u, err := s.Store.Post(ctx, "", data)
	if err != nil {
		return nil, err
	}
	return &Ref{Ref: &Ref_Url{u}}, nil
}

func (s *BasicStore) MaxBlobSize() int {
	return s.Store.MaxBlobSize()
}

func NewMemStore(maxBlobSize int) *BasicStore {
	s := stores.NewMemStore(maxBlobSize)
	return &BasicStore{Store: s}
}

type SpreadStore struct {
	Prefixes []string
	Store    stores.Post
}

func (s *SpreadStore) Post(ctx context.Context, data []byte) (*Ref, error) {
	refs := []*Ref{}
	if len(s.Prefixes) < 1 {
		return nil, errors.New("no prefixes to post to")
	}

	for _, prefix := range s.Prefixes {
		key, err := s.Store.Post(ctx, prefix, data)
		if err != nil {
			return nil, err
		}
		ref := &Ref{
			Ref: &Ref_Url{key},
		}
		refs = append(refs, ref)
	}

	return &Ref{
		Ref: &Ref_Mirror{&Mirror{
			Refs: refs,
		}},
	}, nil
}

func (s *SpreadStore) MaxBlobSize() int {
	return s.Store.MaxBlobSize()
}
