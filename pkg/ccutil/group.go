package ccutil

import (
	"context"
	"fmt"
	"sync"
)

type RunFunc func(context.Context) error

type Group []RunFunc

func (g Group) Run(ctx context.Context) error {
	ctx, cf := context.WithCancel(ctx)
	defer cf()

	wg := sync.WaitGroup{}
	wg.Add(len(g))
	errs := make([]error, len(g))
	for i, f := range g {
		i := i
		f := f
		go func() {
			errs[i] = f(ctx)
			if errs[i] != nil {
				cf()
			}
			wg.Done()
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return fmt.Errorf("%v", errs)
		}
	}
	return nil
}
