package webfsim

import (
	"context"
	"encoding/binary"
	"errors"
	"io"

	"github.com/brendoncarroll/webfs/pkg/stores"
	webref "github.com/brendoncarroll/webfs/pkg/webref"
	wrds "github.com/brendoncarroll/webfs/pkg/wrds"
)

func FileFromReader(ctx context.Context, s stores.ReadPost, opts webref.Options, r io.Reader) (*File, error) {
	buf := make([]byte, s.MaxBlobSize())
	if len(buf) == 0 {
		panic("max blob size 0")
	}

	var (
		size uint64
		done = false
		tb   = wrds.NewTreeBuilder()
	)

	for !done {
		n, err := r.Read(buf)
		if err == io.EOF {
			done = true
		} else if err != nil {
			return nil, err
		}
		data := buf[:n]
		if len(data) < 1 {
			break
		}
		ref, err := webref.Post(ctx, s, opts, data)
		if err != nil {
			return nil, err
		}

		endOffset := size + uint64(len(data))
		ent := &wrds.TreeEntry{
			Key: offset2Key(endOffset),
			Ref: ref,
		}
		if err = tb.Put(ctx, s, opts, ent); err != nil {
			return nil, err
		}
		size = endOffset
	}
	tree, err := tb.Finish(ctx, s, opts)
	if err != nil {
		return nil, err
	}
	f := &File{
		Tree: tree,
		Size: size,
	}
	return f, nil
}

func FileAppend(ctx context.Context, s stores.ReadPost, opts webref.Options, x File, p []byte) (*File, error) {
	var (
		err error
	)
	y := &File{}

	ref, err := webref.Post(ctx, s, opts, p)
	if err != nil {
		return nil, err
	}

	offset := x.Size + uint64(len(p))
	key := offset2Key(offset)
	y.Tree, err = x.Tree.Put(ctx, s, opts, key, ref)
	if err != nil {
		return nil, err
	}
	y.Size = offset
	return y, nil
}

func FileReadAt(ctx context.Context, s stores.Read, x File, off uint64, p []byte) (n int, err error) {
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
		data, err := webref.Get(ctx, s, *ent.Ref)
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

func FileWriteAt(ctx context.Context, s stores.ReadPost, x File) (*File, error) {
	return nil, errors.New("FileWriteAt not implemented")
}

func FileSplit(ctx context.Context, store stores.ReadPost, opts webref.Options, x File) (*File, error) {
	newTree, err := x.Tree.Split(ctx, store, opts)
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
