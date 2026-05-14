package webfs

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"

	capnp "capnproto.org/go/capnp/v3"
	"github.com/brendoncarroll/webfs/src/internal/wfscnp"
	"github.com/gotvc/got/src/gdat"
	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/exp/streams"
	"go.brendoncarroll.net/tai64"
)

type FileInfo struct {
	Size      uint64
	BlockSize uint32
}

type FileParams struct {
	Now       tai64.TAI64N
	BlockSize uint32
	Size      uint64
}

func (tx *Tx) CreateFileAt(ctx context.Context, parent INode, name string, mode fs.FileMode, fp FileParams) (INode, error) {
	ino, err := tx.createFile(ctx, fp, mode)
	if err != nil {
		return INode{}, err
	}
	if err := tx.Link(ctx, parent, name, ino); err != nil {
		return INode{}, err
	}
	return ino, nil
}

func (tx *Tx) createFile(ctx context.Context, fp FileParams, mode fs.FileMode) (INode, error) {
	if fp.BlockSize == 0 {
		return INode{}, fmt.Errorf("block size must be > 0")
	}
	_, seg := capnp.NewSingleSegmentMessage(nil)
	node, err := wfscnp.NewRootNode(seg)
	if err != nil {
		return INode{}, err
	}
	node.SetRefCount(0)
	setNodeMode(node, mode&(fs.ModeType|0o7777))
	createdAt, err := node.NewCreatedAt()
	if err != nil {
		return INode{}, err
	}
	createdAt.SetSeconds(fp.Now.Seconds)
	createdAt.SetNanoseconds(fp.Now.Nanoseconds)
	modifiedAt, err := node.NewModifiedAt()
	if err != nil {
		return INode{}, err
	}
	modifiedAt.SetSeconds(fp.Now.Seconds)
	modifiedAt.SetNanoseconds(fp.Now.Nanoseconds)
	fileInfo, err := node.Payload().NewFile()
	if err != nil {
		return INode{}, err
	}
	fileInfo.SetBlockSize(fp.BlockSize)
	fileInfo.SetSize(fp.Size)

	ino := newINode()
	if err := tx.putNode(ctx, ino, node); err != nil {
		return INode{}, err
	}
	return ino, nil
}

func (tx *Tx) getFile(ctx context.Context, ino INode) (wfscnp.File, wfscnp.Node, error) {
	node, err := tx.getNode(ctx, ino)
	if err != nil {
		return wfscnp.File{}, wfscnp.Node{}, err
	}
	if node.Payload().Which() != wfscnp.Node_payload_Which_file {
		return wfscnp.File{}, wfscnp.Node{}, &ErrWrongType{INode: ino, WantType: "file"}
	}
	file, err := node.Payload().File()
	if err != nil {
		return wfscnp.File{}, wfscnp.Node{}, err
	}
	return file, node, nil
}

func setNodeMode(node wfscnp.Node, mode fs.FileMode) {
	node.SetMode(uint32(mode))
}

func nodeMode(node wfscnp.Node) fs.FileMode {
	mode := fs.FileMode(node.Mode())
	if mode != 0 {
		return mode
	}
	switch node.Payload().Which() {
	case wfscnp.Node_payload_Which_dir:
		return fs.ModeDir | 0o755
	case wfscnp.Node_payload_Which_file:
		return 0o644
	default:
		return 0o644
	}
}

func (tx *Tx) GetMode(ctx context.Context, ino INode) (fs.FileMode, error) {
	node, err := tx.getNode(ctx, ino)
	if err != nil {
		return 0, err
	}
	mode := nodeMode(node)
	if node.Payload().Which() == wfscnp.Node_payload_Which_dir {
		mode |= fs.ModeDir
	}
	return mode, nil
}

func (tx *Tx) SetMode(ctx context.Context, ino INode, mode fs.FileMode) error {
	node, err := tx.getNode(ctx, ino)
	if err != nil {
		return err
	}
	if node.Payload().Which() == wfscnp.Node_payload_Which_dir {
		mode |= fs.ModeDir
	}
	setNodeMode(node, mode)
	return tx.putNode(ctx, ino, node)
}

func (tx *Tx) StatFile(ctx context.Context, ino INode) (FileInfo, error) {
	file, _, err := tx.getFile(ctx, ino)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{Size: file.Size(), BlockSize: file.BlockSize()}, nil
}

func (tx *Tx) Truncate(ctx context.Context, ino INode, size uint64) error {
	file, node, err := tx.getFile(ctx, ino)
	if err != nil {
		return err
	}
	if size == file.Size() {
		return nil
	}
	if size == 0 {
		if _, err := tx.inodetx.Flush(ctx); err != nil {
			return err
		}
		it := tx.inodetx.IterateFlushed(ctx, gotkv.PrefixSpan(ino[:]))
		buf := make([]gotkv.Entry, 32)
		for {
			n, err := it.Next(ctx, buf)
			if err != nil {
				if streams.IsEOS(err) {
					break
				}
				return err
			}
			for i := 0; i < n; i++ {
				if len(buf[i].Key) != len(ino)+8 {
					continue
				}
				if err := tx.inodetx.Delete(ctx, buf[i].Key); err != nil {
					return err
				}
			}
		}
	}
	file.SetSize(size)
	return tx.putNode(ctx, ino, node)
}

// WriteBlock writes buf, whos length must match the file's block size to the file.
// Writing an all zero block deletes the entry for the block.
func (tx *Tx) WriteBlock(ctx context.Context, ino INode, blockno uint64, buf []byte) error {
	file, _, err := tx.getFile(ctx, ino)
	if err != nil {
		return err
	}
	if int(file.BlockSize()) != len(buf) {
		return fmt.Errorf("buffer length is different from block size HAVE: %d WANT: %d", len(buf), file.BlockSize())
	}
	return tx.writeBlock(ctx, ino, blockno, buf)
}

func (tx *Tx) writeBlock(ctx context.Context, ino INode, blockno uint64, buf []byte) error {
	key := makeBlockKey(nil, ino, blockno)
	if isAllZero(buf) {
		return tx.inodetx.Delete(ctx, key)
	}
	ref, err := tx.fdata.Post(ctx, tx.rws, buf)
	if err != nil {
		return err
	}
	return tx.inodetx.Put(ctx, key, ref.Marshal())
}

// ReadBlock reads the blockno into buf.  If buf is short, data is written until buf is full, then nil is returned.
// If there is no block
func (tx *Tx) ReadBlock(ctx context.Context, ino INode, blockno uint64, buf []byte) error {
	file, _, err := tx.getFile(ctx, ino)
	if err != nil {
		return err
	}
	return tx.readBlock(ctx, ino, blockno, file.BlockSize(), buf)
}

// readBlock assumes that ino refers to a valid file node in this transaction
func (tx *Tx) readBlock(ctx context.Context, ino INode, blockno uint64, blockSize uint32, buf []byte) error {
	key := makeBlockKey(nil, ino, blockno)
	var refData []byte
	exists, err := tx.inodetx.Get(ctx, key, &refData)
	if err != nil {
		return err
	}
	if !exists {
		clear(buf)
		return nil
	}
	var ref gdat.Ref
	if err := ref.Unmarshal(refData); err != nil {
		return err
	}
	readBuf := buf
	if int(blockSize) > len(readBuf) {
		readBuf = make([]byte, blockSize)
	}
	n, err := tx.fdata.Read(ctx, tx.ros, ref, readBuf)
	if err != nil {
		return err
	}
	clear(readBuf[n:])
	if len(readBuf) != len(buf) {
		copy(buf, readBuf[:len(buf)])
	}
	return nil
}

// WriteAt writes the contents of buf, starting at offset within the file.
func (tx *Tx) WriteAt(ctx context.Context, ino INode, offset int64, buf []byte) error {
	if offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}
	if len(buf) == 0 {
		return nil
	}
	file, node, err := tx.getFile(ctx, ino)
	if err != nil {
		return err
	}
	blockSize := int64(file.BlockSize())
	if blockSize <= 0 {
		return fmt.Errorf("inode %v has invalid block size %d", ino, blockSize)
	}

	remaining := buf
	curOff := offset
	for len(remaining) > 0 {
		blockNo := uint64(curOff / blockSize)
		blockOff := int(curOff % blockSize)
		toWrite := min(len(remaining), int(blockSize)-blockOff)
		if blockOff == 0 && toWrite == int(blockSize) {
			if err := tx.writeBlock(ctx, ino, blockNo, remaining[:toWrite]); err != nil {
				return err
			}
		} else {
			fullBlock := make([]byte, int(blockSize))
			if err := tx.readBlock(ctx, ino, blockNo, uint32(blockSize), fullBlock); err != nil {
				return err
			}
			copy(fullBlock[blockOff:blockOff+toWrite], remaining[:toWrite])
			if err := tx.writeBlock(ctx, ino, blockNo, fullBlock); err != nil {
				return err
			}
		}
		remaining = remaining[toWrite:]
		curOff += int64(toWrite)
	}
	end := uint64(offset) + uint64(len(buf))
	if end > file.Size() {
		file.SetSize(end)
		if err := tx.putNode(ctx, ino, node); err != nil {
			return err
		}
	}
	return nil
}

func (tx *Tx) ReadAt(ctx context.Context, ino INode, offset int64, buf []byte) (int, error) {
	if offset < 0 {
		return 0, fmt.Errorf("offset cannot be negative")
	}
	if len(buf) == 0 {
		return 0, nil
	}
	finfo, _, err := tx.getFile(ctx, ino)
	if err != nil {
		return 0, err
	}
	if uint64(offset) >= finfo.Size() {
		return 0, io.EOF
	}
	blockSize := int64(finfo.BlockSize())
	if blockSize <= 0 {
		return 0, fmt.Errorf("inode %v has invalid block size %d", ino, blockSize)
	}
	toRead := min(int64(len(buf)), int64(finfo.Size()-uint64(offset)))
	remaining := int(toRead)
	read := 0
	curOff := offset
	for remaining > 0 {
		blockNo := uint64(curOff / blockSize)
		blockOff := int(curOff % blockSize)
		take := min(remaining, int(blockSize)-blockOff)
		if blockOff == 0 {
			if err := tx.readBlock(ctx, ino, blockNo, uint32(blockSize), buf[read:read+take]); err != nil {
				return read, err
			}
		} else {
			fullBlock := make([]byte, int(blockSize))
			if err := tx.readBlock(ctx, ino, blockNo, uint32(blockSize), fullBlock); err != nil {
				return read, err
			}
			copy(buf[read:read+take], fullBlock[blockOff:blockOff+take])
		}
		read += take
		remaining -= take
		curOff += int64(take)
	}
	if read < len(buf) {
		if read == 0 {
			return 0, io.EOF
		}
		return read, nil
	}
	return read, nil
}

func makeBlockKey(key []byte, ino INode, blockno uint64) []byte {
	key = append(key, ino[:]...)
	key = binary.BigEndian.AppendUint64(key, blockno)
	return key
}

func isAllZero(x []byte) bool {
	for i := range x {
		if x[i] != 0 {
			return false
		}
	}
	return true
}
