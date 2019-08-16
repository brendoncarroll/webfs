package webfs

import (
	"context"
	"errors"
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"

	"github.com/brendoncarroll/webfs/pkg/stores/cahttp"
	"github.com/brendoncarroll/webfs/pkg/stores/ipfsstore"
)

type Store struct {
	parent *Store
	router *stores.Router
}

func newStore(parent *Store, specs []*models.StoreSpec) (*Store, error) {
	routes := make([]stores.StoreRoute, len(specs))
	for i, spec := range specs {
		var s stores.ReadWriteOnce
		switch x := spec.Spec.(type) {
		case *models.StoreSpec_Cahttp:
			s = cahttp.New(x.Cahttp.Endpoint, x.Cahttp.Prefix)
		case *models.StoreSpec_Ipfs:
			s = ipfsstore.New(x.Ipfs.Endpoint)
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
	return x
}
