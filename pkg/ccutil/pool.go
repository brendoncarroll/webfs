package ccutil

import "context"

type WorkerFunc func(ctx context.Context, i int) error

type Pool struct {
	N int
	F WorkerFunc
}

func (p *Pool) Run(ctx context.Context) error {
	n := p.N
	errs := make(chan error, n)
	defer close(errs)

	ctx, cf := context.WithCancel(ctx)
	defer cf()

	for i := 0; i < n; i++ {
		i := i
		go func() {
			errs <- p.F(ctx, i)
		}()
	}

	var retErr error
	for i := 0; i < n; i++ {
		err := <-errs
		if err != nil {
			if retErr == nil {
				retErr = err
				cf()
			}
		}
	}

	return retErr
}
