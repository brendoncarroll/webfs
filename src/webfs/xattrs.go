package webfs

import (
	"context"
	"fmt"
	"io/fs"
	"math"

	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/exp/streams"
)

const (
	MaxXAttrKeySize   = 255
	MaxXAttrValueSize = math.MaxUint16
)

type XAttrKey string

func (tx *Tx) SetXAttr(ctx context.Context, ino INode, key XAttrKey, value []byte) error {
	if err := checkXAttrKey(key); err != nil {
		return err
	}
	if len(value) > MaxXAttrValueSize {
		return fmt.Errorf("xattr value is too large HAVE: %d WANT <= %d", len(value), MaxXAttrValueSize)
	}
	if _, err := tx.getNode(ctx, ino); err != nil {
		return err
	}
	return tx.xattrtx.Put(ctx, makeXAttrKey(nil, ino, key), value)
}

func (tx *Tx) GetXAttr(ctx context.Context, ino INode, key XAttrKey) ([]byte, error) {
	if err := checkXAttrKey(key); err != nil {
		return nil, err
	}
	if _, err := tx.getNode(ctx, ino); err != nil {
		return nil, err
	}
	var value []byte
	exists, err := tx.xattrtx.Get(ctx, makeXAttrKey(nil, ino, key), &value)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fs.ErrNotExist
	}
	return value, nil
}

func (tx *Tx) RemoveXAttr(ctx context.Context, ino INode, key XAttrKey) error {
	if err := checkXAttrKey(key); err != nil {
		return err
	}
	if _, err := tx.getNode(ctx, ino); err != nil {
		return err
	}
	return tx.xattrtx.Delete(ctx, makeXAttrKey(nil, ino, key))
}

func (tx *Tx) ListXAttrs(ctx context.Context, ino INode) ([]XAttrKey, error) {
	if _, err := tx.getNode(ctx, ino); err != nil {
		return nil, err
	}
	if tx.xattrtx.Queued() > 0 {
		if _, err := tx.xattrtx.Flush(ctx); err != nil {
			return nil, err
		}
	}
	it := tx.xattrtx.Iterate(ctx, gotkv.PrefixSpan(ino[:]))
	buf := make([]gotkv.Entry, 32)
	var keys []XAttrKey
	for {
		n, err := it.Next(ctx, buf)
		if err != nil {
			if streams.IsEOS(err) {
				return keys, nil
			}
			return nil, err
		}
		for i := 0; i < n; i++ {
			keys = append(keys, XAttrKey(string(buf[i].Key[len(ino):])))
		}
	}
}

func makeXAttrKey(out []byte, ino INode, key XAttrKey) []byte {
	out = append(out, ino[:]...)
	out = append(out, key...)
	return out
}

func checkXAttrKey(key XAttrKey) error {
	if len(key) > MaxXAttrKeySize {
		return fmt.Errorf("xattr key is too large HAVE: %d WANT <= %d", len(key), MaxXAttrKeySize)
	}
	return nil
}

func (tx *Tx) deleteXAttrs(ctx context.Context, ino INode) error {
	if _, err := tx.xattrtx.Flush(ctx); err != nil {
		return err
	}
	it := tx.xattrtx.IterateFlushed(ctx, gotkv.PrefixSpan(ino[:]))
	buf := make([]gotkv.Entry, 32)
	for {
		n, err := it.Next(ctx, buf)
		if err != nil {
			if streams.IsEOS(err) {
				return nil
			}
			return err
		}
		for i := 0; i < n; i++ {
			// TODO: remove when fixed upstream; gotkv.Tx.Delete retains the key slice.
			if err := tx.xattrtx.Delete(ctx, append([]byte(nil), buf[i].Key...)); err != nil {
				return err
			}
		}
	}
}
