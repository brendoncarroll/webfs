package webfs

import (
	"context"
	"encoding/binary"
	"fmt"
)

func (tx *Tx) InspectFile(ctx context.Context, ino INode) (File, error) {
	return File{}, nil
}

// WriteBlock writes buf, whos length must match the file's block size to the file.
func (tx *Tx) WriteBlock(ctx context.Context, ino INode, blockno uint64, buf []byte) error {
	finfo, err := tx.InspectFile(ctx, ino)
	if err != nil {
		return err
	}
	if int(finfo.BlockSize) != len(buf) {
		return fmt.Errorf("buffer length is different from block size HAVE: %d WANT: %d", len(buf), finfo.BlockSize)
	}
	return tx.writeBlock(ctx, ino, blockno, buf)
}

func (tx *Tx) writeBlock(ctx context.Context, ino INode, blockno uint64, buf []byte) error {
	ref, err := tx.machs.fdata.Post(ctx, tx.rws, buf)
	if err != nil {
		return err
	}
	key := makeBlockKey(nil, ino, blockno)
	return tx.inodetx.Put(ctx, key, ref.Marshal())
}

// ReadBlock reads the blockno into buf.  If buf is short, data is written until buf is full, then nil is returned.
// If there is no block
func (tx *Tx) ReadBlock(ctx context.Context, ino INode, blockno uint64, buf []byte) error {
	return nil
}

// WriteAt writes the contents of buf, starting at offset within the file.
func (tx *Tx) WriteAt(ctx context.Context, ino INode, offset int64, buf []byte) error {
	return nil
}

func (tx *Tx) ReadAt(ctx context.Context, ino INode, offset int64, buf []byte) (int, error) {
	return 0, nil
}

func makeBlockKey(key []byte, ino INode, blockno uint64) []byte {
	key = append(key, ino[:]...)
	key = binary.BigEndian.AppendUint64(key, blockno)
	return key
}
