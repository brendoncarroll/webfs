package webfsim

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"sync"

	"github.com/brendoncarroll/webfs/pkg/ccutil"
	webref "github.com/brendoncarroll/webfs/pkg/webref"
	wrds "github.com/brendoncarroll/webfs/pkg/wrds"
)

type dataPair struct {
	Data   []byte
	Offset uint64
}

type refPair struct {
	Ref    *webref.Ref
	Offset uint64
}

func FileFromReader(ctx context.Context, s ReadPost, r io.Reader) (*File, error) {
	const postWorkers = 10
	bufPool := sync.Pool{
		New: func() interface{} {
			buf := make([]byte, s.MaxBlobSize())
			if len(buf) == 0 {
				panic("max blob size 0")
			}
			return buf
		},
	}

	var (
		dataPairs = make(chan dataPair, 16)
		refPairs  = make(chan refPair)
		f         *File
	)

	group := ccutil.Group{
		// read from stream
		func(ctx context.Context) error {
			defer close(dataPairs)

			done := false
			size := uint64(0)
			for !done {
				buf := bufPool.Get().([]byte)[:s.MaxBlobSize()]
				data, err := readUntilFull(r, buf)
				if err == io.EOF {
					done = true
				} else if err != nil {
					return err
				}

				if len(data) < 1 {
					continue
				}

				endOffset := size + uint64(len(data))
				dp := dataPair{
					Data:   data,
					Offset: endOffset,
				}
				dataPairs <- dp

				size += uint64(len(data))
			}
			return nil
		},

		// post workers
		func(ctx context.Context) error {
			oq := ccutil.OrderedQueue{
				In:  make(chan interface{}),
				Out: make(chan interface{}),
				N:   postWorkers,
				F: func(ctx context.Context, x interface{}) (interface{}, error) {
					pair := x.(dataPair)
					ref, err := s.Post(ctx, pair.Data)
					if err != nil {
						return nil, err
					}

					// return buffer
					buf := pair.Data[:cap(pair.Data)]
					bufPool.Put(buf)

					rp := refPair{
						Offset: pair.Offset,
						Ref:    ref,
					}
					return rp, nil
				},
			}

			group := ccutil.Group{
				func(ctx context.Context) error {
					for x := range dataPairs {
						oq.In <- x
					}
					close(oq.In)
					return nil
				},
				oq.Run,
				func(ctx context.Context) error {
					for x := range oq.Out {
						refPairs <- x.(refPair)
					}
					close(refPairs)
					return nil
				},
			}

			return group.Run(ctx)
		},

		// assemble tree
		func(ctx context.Context) error {
			size := uint64(0)
			tb := wrds.NewTreeBuilder()
			for pair := range refPairs {
				ent := &wrds.TreeEntry{
					Key: offset2Key(pair.Offset),
					Ref: pair.Ref,
				}
				if err := tb.Put(ctx, s, ent); err != nil {
					return err
				}
				size = pair.Offset
			}
			tree, err := tb.Finish(ctx, s)
			if err != nil {
				return err
			}
			f = &File{
				Tree: tree,
				Size: size,
			}
			return nil
		},
	}

	if err := group.Run(ctx); err != nil {
		return nil, err
	}
	return f, nil
}

func FileAppend(ctx context.Context, s ReadPost, x File, p []byte) (*File, error) {
	var (
		err error
	)
	y := &File{}

	ref, err := s.Post(ctx, p)
	if err != nil {
		return nil, err
	}

	offset := x.Size + uint64(len(p))
	key := offset2Key(offset)
	y.Tree, err = x.Tree.Put(ctx, s, key, ref)
	if err != nil {
		return nil, err
	}
	y.Size = offset
	return y, nil
}

func FileReadAt(ctx context.Context, s Getter, x File, off uint64, p []byte) (n int, err error) {
	for n < len(p) {
		ent, err := x.Tree.MinGt(ctx, s, offset2Key(off))
		if err != nil {
			return 0, err
		}
		if ent == nil {
			// done, no entry for this offset, empty file
			return n, io.EOF
		}

		endOffset := key2Offset(ent.Key)
		if endOffset <= off {
			return 0, errors.New("got wrong entry from tree")
		}
		data, err := s.Get(ctx, ent.Ref)
		if err != nil {
			return n, err
		}

		dist2end := endOffset - off
		if dist2end > uint64(len(data)) {
			keys := []uint64{}
			for _, e := range x.Tree.Entries {
				z := key2Offset(e.Key)
				keys = append(keys, z)
			}
			return 0, errors.New("got wrong entry from tree")
		}

		// calculate distance from end
		i := uint64(len(data)) - (endOffset - off)
		n2 := copy(p[n:], data[i:])
		n += n2
		off += uint64(n2)
	}

	return n, nil
}

func FileWriteAt(ctx context.Context, s ReadPost, x File) (*File, error) {
	return nil, errors.New("FileWriteAt not implemented")
}

func FileSplit(ctx context.Context, store ReadPost, x File) (*File, error) {
	newTree, err := x.Tree.Split(ctx, store)
	if err != nil {
		return nil, err
	}
	y := x
	y.Tree = newTree
	return &y, nil
}

func offset2Key(x uint64) []byte {
	keyBuf := [8]byte{}
	binary.BigEndian.PutUint64(keyBuf[:], x)
	return keyBuf[:]
}

func key2Offset(x []byte) uint64 {
	return binary.BigEndian.Uint64(x)
}

func readUntilFull(r io.Reader, buf []byte) ([]byte, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err == io.EOF {
			return buf[:total], io.EOF
		}
		if err != nil {
			return nil, err
		}
	}
	return buf[:total], nil
}
