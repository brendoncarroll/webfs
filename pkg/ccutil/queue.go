package ccutil

import (
	"context"
	"sync"
)

type MapFunc func(ctx context.Context, x interface{}) (interface{}, error)

type Queue struct {
	In, Out chan interface{}
	F       MapFunc
	N       int
}

func (q *Queue) Run(ctx context.Context) error {
	pool := Pool{
		N: q.N,
		F: func(ctx context.Context, i int) error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case x, ok := <-q.In:
					if !ok {
						return nil
					}
					y, err := q.F(ctx, x)
					if err != nil {
						return err
					}
					q.Out <- y
				}
			}
		},
	}
	defer close(q.Out)
	return pool.Run(ctx)
}

type orderedItem struct {
	i     int
	value interface{}
}

type OrderedQueue struct {
	In, Out chan interface{}
	F       MapFunc
	N       int
}

func (q *OrderedQueue) Run(ctx context.Context) error {
	cu := NewCountUp()
	q1 := Queue{
		In:  make(chan interface{}),
		Out: make(chan interface{}),
		N:   q.N,
		F: func(ctx context.Context, x interface{}) (interface{}, error) {
			xoi := x.(orderedItem)
			y, err := q.F(ctx, xoi.value)
			if err != nil {
				return nil, err
			}
			cu.Wait(xoi.i)
			return orderedItem{i: xoi.i, value: y}, nil
		},
	}

	group := Group{
		// created ordered items
		func(ctx context.Context) error {
			i := 0
			for x := range q.In {
				item := orderedItem{
					i:     i,
					value: x,
				}
				q1.In <- item
				i++
			}
			close(q1.In)
			return nil
		},
		q1.Run,
		// unwrap ordered items
		func(ctx context.Context) error {
			for item := range q1.Out {
				oi := item.(orderedItem)
				cu.Done(oi.i)
				q.Out <- oi.value
			}
			close(q.Out)
			return nil
		},
	}

	return group.Run(ctx)
}

type CountUp struct {
	next  int
	mu    sync.Mutex
	chans map[int]chan struct{}
}

func NewCountUp() *CountUp {
	return &CountUp{
		next: 0,
		chans: map[int]chan struct{}{
			0: make(chan struct{}),
		},
	}
}

func (cu *CountUp) Done(n int) {
	cu.mu.Lock()
	defer cu.mu.Unlock()
	if n != cu.next {
		panic("skipped one")
	}
	for i := n; i <= n+1; i++ {
		ch, exists := cu.chans[i]
		if exists {
			close(ch)
			delete(cu.chans, i)
		}
	}
	cu.next++
}

func (cu *CountUp) Wait(n int) {
	cu.mu.Lock()
	if n <= cu.next {
		cu.mu.Unlock()
		return
	}

	ch, exists := cu.chans[n]
	if !exists {
		ch = make(chan struct{})
		cu.chans[n] = ch
	}
	cu.mu.Unlock()

	<-ch
}
