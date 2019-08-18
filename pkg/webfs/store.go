package webfs

import (
	"context"
	"errors"
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"

	"github.com/brendoncarroll/webfs/pkg/stores/httpstore"
	"github.com/brendoncarroll/webfs/pkg/stores/ipfsstore"
)

type Store struct {
	parent *Store
	router *stores.Router
}

func newStore(parent *Store, specs []*models.StoreSpec) (*Store, error) {
	var err error
	routes := make([]stores.StoreRoute, len(specs))
	for i, spec := range specs {
		var s stores.ReadWriteOnce
		switch x := spec.Spec.(type) {
		case *models.StoreSpec_Http:
			s = httpstore.New(x.Http.Endpoint, x.Http.Prefix)
		case *models.StoreSpec_Ipfs:
			s, err = ipfsstore.New(x.Ipfs.Endpoint)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("bad spec %v", spec)
		}

		routes[i] = stores.StoreRoute{
			Prefix: spec.Prefix,
			Store:  s,
		}
	}

	return &Store{
		parent: parent,
		router: stores.NewRouter(routes),
	}, nil
}

func (s *Store) getStore(key string) stores.Read {
	store := s.router.LookupStore(key)
	switch {
	case store != nil:
		return store
	case store == nil && s.parent != nil:
		return s.parent.getStore(key)
	default:
		return nil
	}
}

func (s *Store) getWriteStore(prefix string) (stores.WriteOnce, error) {
	store := s.getStore(prefix)
	if wstore, ok := store.(stores.ReadWriteOnce); ok {
		return wstore, nil
	}
	return nil, errors.New("no writeable store")
}

func (s *Store) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	store, err := s.getWriteStore(prefix)
	if err != nil {
		return "", err
	}
	return store.Post(ctx, prefix, data)
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	return s.getStore(key).Get(ctx, key)
}

func (s *Store) MaxBlobSize() int {
	x := s.router.MaxBlobSize()
	if x == 0 && s.parent != nil {
		return s.parent.MaxBlobSize()
	}
	if s.parent == nil {
		return x
	}
	parentMbs := s.parent.MaxBlobSize()
	if parentMbs == 0 {
		return x
	}
	return min(x, parentMbs)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
