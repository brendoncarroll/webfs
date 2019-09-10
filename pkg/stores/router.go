package stores

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrNoRoute  = errors.New("no store for prefix")
	ErrNoPost   = errors.New("store does not support post")
	ErrNoDelete = errors.New("store does not support delete")
)

type StoreRoute struct {
	Prefix string
	Store  Read
}

type Router struct {
	routes []StoreRoute
}

func NewRouter(routes []StoreRoute) *Router {
	r := &Router{
		routes: routes,
	}

	return r
}

func (r *Router) Get(ctx context.Context, key string) ([]byte, error) {
	s := r.LookupStore(key)
	if s == nil {
		return nil, ErrNoRoute
	}
	return s.Get(ctx, key)
}

func (r *Router) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	s := r.LookupStore(prefix)
	if s == nil {
		return "", ErrNoRoute
	}
	w, ok := s.(Post)
	if !ok {
		return "", ErrNoPost
	}
	return w.Post(ctx, prefix, data)
}

func (r *Router) Delete(ctx context.Context, key string) error {
	s := r.LookupStore(key)
	if s == nil {
		return ErrNoRoute
	}
	w, ok := s.(Delete)
	if !ok {
		return ErrNoDelete
	}
	return w.Delete(ctx, key)
}

func (r *Router) Check(ctx context.Context, key string) error {
	s := r.LookupStore(key)
	if s == nil {
		return ErrNoRoute
	}
	c, ok := s.(Check)
	if !ok {
		_, err := s.Get(ctx, key)
		return err
	}
	return c.Check(ctx, key)
}

func (r *Router) AppendWith(b *Router) {
	r.routes = append(r.routes, b.routes...)
}

func (r *Router) LookupStore(key string) Read {
	for _, route := range r.routes {
		if strings.HasPrefix(key, route.Prefix) {
			return route.Store
		}
	}
	return nil
}

func (r *Router) MaxBlobSize() int {
	maxSize := 0
	for _, route := range r.routes {
		wstore, ok := route.Store.(ReadPost)
		if ok && wstore != nil {
			ms2 := wstore.MaxBlobSize()
			if maxSize == 0 || ms2 < maxSize {
				maxSize = ms2
			}
		}
	}
	return maxSize
}
