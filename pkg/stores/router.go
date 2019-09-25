package stores

import (
	"errors"
	"strings"
)

var (
	ErrNoRoute = errors.New("no store route for prefix")
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
			if ms2 > maxSize {
				maxSize = ms2
			}
		}
	}
	return maxSize
}
