package webfs

import (
	"context"
	"fmt"
	"log"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/brendoncarroll/webfs/pkg/stores/httpstore"
	"github.com/brendoncarroll/webfs/pkg/stores/ipfsstore"
	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Store struct {
	get    webref.Getter
	post   webref.Poster
	router *stores.Router
}

func (s *Store) Get(ctx context.Context, ref *webref.Ref) ([]byte, error) {
	return s.get.Get(ctx, ref)
}

func (s *Store) Post(ctx context.Context, data []byte) (*Ref, error) {
	return s.post.Post(ctx, data)
}

func (s *Store) MaxBlobSize() int {
	return s.post.MaxBlobSize()
}

func (s *Store) Check(ctx context.Context, ref *webref.Ref) []webref.RefStatus {
	checker, ok := s.get.(webref.Checker)
	if !ok {
		_, err := s.get.Get(ctx, ref)
		return []webref.RefStatus{
			{Error: err},
		}
	}
	return checker.Check(ctx, ref)
}

func (s *Store) Delete(ctx context.Context, ref *webref.Ref) error {
	deleter, ok := s.post.(webref.Deleter)
	if !ok {
		return nil
	}
	return deleter.Delete(ctx, ref)
}

func BuildStore(specs []*webfsim.StoreSpec, o *webfsim.WriteOptions) (*Store, error) {
	router, err := newRouter(specs)
	if err != nil {
		return nil, err
	}
	prefixes := []string{}
	for prefix, n := range o.Replicas {
		prefixes = append(prefixes, prefix)
		if n > 1 {
			log.Println("WARN: multiple replicas per prefix not yet supported")
		}
	}
	s := &Store{
		router: router,
		post: &webref.CryptoStore{
			Inner: &webref.SpreadStore{
				Store:    router,
				Prefixes: prefixes,
			},
			EncAlgo:    o.EncAlgo,
			SecretSeed: o.SecretSeed,
		},
		get: webref.NewCache(&webref.BasicStore{Store: router}),
	}

	return s, nil
}

func newRouter(specs []*webfsim.StoreSpec) (*stores.Router, error) {
	routes := make([]stores.StoreRoute, len(specs))
	for i, spec := range specs {
		var s stores.ReadPost
		var err error
		switch x := spec.Spec.(type) {
		case *webfsim.StoreSpec_Http:
			s, err = httpstore.New(x.Http.Endpoint, x.Http.Prefix, x.Http.Headers)
		case *webfsim.StoreSpec_Ipfs:
			s = ipfsstore.New(x.Ipfs.Endpoint)
		default:
			return nil, fmt.Errorf("bad spec %v", spec)
		}
		if err != nil {
			return nil, err
		}

		routes[i] = stores.StoreRoute{
			Prefix: spec.Prefix,
			Store:  s,
		}
	}
	return stores.NewRouter(routes), nil
}
