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
	"go.brendoncarroll.net/exp/sbe"
	"go.brendoncarroll.net/exp/streams"
	"go.brendoncarroll.net/tai64"
)

type FileInfo struct {
	Size uint64
}

type Extent struct {
	Ref gdat.Ref
	Len uint32
}

func (ext Extent) Marshal(out []byte) []byte {
	out = append(out, ext.Ref.Marshal()...)
	out = sbe.AppendUint32(out, ext.Len)
	return out
}

func (ext *Extent) Unmarshal(data []byte) error {
	refData, data, err := sbe.ReadN(data, gdat.RefSize)
	if err != nil {
		return err
	}
	if err := ext.Ref.Unmarshal(refData); err != nil {
		return err
	}
	l, _, err := sbe.ReadUint32(data)
	if err != nil {
		return err
	}
	ext.Len = l
	return nil
}

type FileParams struct {
	Now       tai64.TAI64N
	BlockSize uint32
	Size      uint64
}

func (tx *FSTx) CreateFileAt(ctx context.Context, parent INode, name string, mode fs.FileMode, fp FileParams) (INode, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	ino, err := tx.createFile(ctx, fp, mode)
	if err != nil {
		return INode{}, err
	}
	if err := tx.link(ctx, parent, name, ino); err != nil {
		return INode{}, err
	}
	return ino, nil
}

// createFile assumes the lock is held
func (tx *FSTx) createFile(ctx context.Context, fp FileParams, mode fs.FileMode) (INode, error) {
	// assumes lock is held
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

// getFile assumes the lock is held
func (tx *FSTx) getFile(ctx context.Context, ino INode) (wfscnp.File, wfscnp.Node, error) {
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

// lockAndGetFile acquires the lock and then calls getFileLocked
func (tx *FSTx) lockAndGetFile(ctx context.Context, ino INode) (wfscnp.File, wfscnp.Node, error) {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.getFile(ctx, ino)
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

func (tx *FSTx) GetMode(ctx context.Context, ino INode) (fs.FileMode, error) {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.getMode(ctx, ino)
}

func (tx *FSTx) getMode(ctx context.Context, ino INode) (fs.FileMode, error) {
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

func (tx *FSTx) SetMode(ctx context.Context, ino INode, mode fs.FileMode) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
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

func (tx *FSTx) StatFile(ctx context.Context, ino INode) (FileInfo, error) {
	file, _, err := tx.lockAndGetFile(ctx, ino)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{Size: file.Size()}, nil
}

func (tx *FSTx) Truncate(ctx context.Context, ino INode, size uint64) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
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

func extKey(ino INode, endAt uint64) [16 + 8]byte {
	var k [16 + 8]byte
	copy(k[:16], ino[:])
	binary.BigEndian.PutUint64(k[16:], endAt)
	return k
}

// putExtent inserts/updates an Extent ending at endAt
// putExtent assumes the write lock is held.
func (tx *FSTx) putExtent(ctx context.Context, ino INode, endAt uint64, ext Extent) error {
	k := extKey(ino, endAt)
	v := ext.Marshal(nil)
	return tx.inodetx.Put(ctx, k[:], v)
}

// getExtent retreives an extent from
// getExtent assumes a lock is held.
func (tx *FSTx) getExtent(ctx context.Context, ino INode, endAt uint64) (Extent, error) {
	k := extKey(ino, endAt)
	var val []byte
	if exists, err := tx.inodetx.Get(ctx, k[:], &val); err != nil {
		return Extent{}, err
	} else if !exists {
		return Extent{}, fmt.Errorf("extent not found")
	}
	var ext Extent
	return ext, ext.Unmarshal(val)
}

// writeExtent assumes the write lock is held.
func (tx *FSTx) writeExtent(ctx context.Context, ino INode, endAt uint64, data []byte) error {
	if len(data) == 0 || isAllZero(data) {
		return nil
	}
	if uint64(len(data)) > endAt {
		return fmt.Errorf("extent length %d exceeds end offset %d", len(data), endAt)
	}
	if uint64(len(data)) > uint64(^uint32(0)) {
		return fmt.Errorf("extent length %d exceeds max length %d", len(data), uint64(^uint32(0)))
	}
	ref, err := tx.fdata.Post(ctx, tx.rws, data)
	if err != nil {
		return err
	}
	return tx.putExtent(ctx, ino, endAt, Extent{Ref: ref, Len: uint32(len(data))})
}

// readExtent call fn with data from the extent
// readExtent can be called without the read lock
func (tx *FSTx) readExtent(ctx context.Context, ext Extent, fn func([]byte) error) error {
	return tx.fdata.GetF(ctx, tx.ros, ext.Ref, fn)
}

// getOverlappingExtents assumes the lock is held. If pending inode edits exist,
// it flushes them so the range scan sees the current transaction state.
func (tx *FSTx) getOverlappingExtents(ctx context.Context, ino INode, start, end uint64) ([]extentEntry, error) {
	if start >= end {
		return nil, nil
	}
	if tx.inodetx.Queued() > 0 {
		if _, err := tx.inodetx.Flush(ctx); err != nil {
			return nil, err
		}
	}
	begin := extKey(ino, start+1)
	it := tx.inodetx.IterateFlushed(ctx, gotkv.Span{Begin: begin[:], End: gotkv.PrefixEnd(ino[:])})
	buf := make([]gotkv.Entry, 32)
	var ret []extentEntry
	for {
		n, err := it.Next(ctx, buf)
		if err != nil {
			if streams.IsEOS(err) {
				return ret, nil
			}
			return nil, err
		}
		for i := 0; i < n; i++ {
			ee, err := parseExtentEntry(buf[i])
			if err != nil {
				return nil, err
			}
			extStart := ee.startAt()
			if extStart >= end {
				continue
			}
			ret = append(ret, ee)
		}
	}
}

// writeExtents writes data across as many extents as needed.
func (tx *FSTx) writeExtents(ctx context.Context, ino INode, start uint64, data []byte) error {
	maxChunk := tx.rws.MaxSize()
	if maxChunk <= 0 {
		return fmt.Errorf("invalid max blob size %d", maxChunk)
	}
	if uint64(maxChunk) > uint64(^uint32(0)) {
		maxChunk = int(^uint32(0))
	}
	for len(data) > 0 {
		n := min(len(data), maxChunk)
		end := start + uint64(n)
		if err := tx.writeExtent(ctx, ino, end, data[:n]); err != nil {
			return err
		}
		start = end
		data = data[n:]
	}
	return nil
}

// WriteAt writes the contents of buf, starting at offset within the file.
func (tx *FSTx) WriteAt(ctx context.Context, ino INode, offset int64, buf []byte) error {
	if offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}
	if len(buf) == 0 {
		return nil
	}
	start := uint64(offset)
	if start > ^uint64(0)-uint64(len(buf)) {
		return fmt.Errorf("write range overflows uint64")
	}
	end := start + uint64(len(buf))

	tx.mu.Lock()
	defer tx.mu.Unlock()
	if _, _, err := tx.getFile(ctx, ino); err != nil {
		return err
	}
	overlaps, err := tx.getOverlappingExtents(ctx, ino, start, end)
	if err != nil {
		return err
	}
	for _, ee := range overlaps {
		extStart := ee.startAt()
		var data []byte
		if err := tx.readExtent(ctx, ee.Ext, func(x []byte) error {
			if len(x) < int(ee.Ext.Len) {
				return fmt.Errorf("extent data is shorter than declared length: %d < %d", len(x), ee.Ext.Len)
			}
			data = append(data, x[:ee.Ext.Len]...)
			return nil
		}); err != nil {
			return err
		}
		key := extKey(ino, ee.EndAt)
		if err := tx.inodetx.Delete(ctx, key[:]); err != nil {
			return err
		}
		if extStart < start {
			if err := tx.writeExtents(ctx, ino, extStart, data[:start-extStart]); err != nil {
				return err
			}
		}
		if ee.EndAt > end {
			if err := tx.writeExtents(ctx, ino, end, data[end-extStart:]); err != nil {
				return err
			}
		}
	}
	if err := tx.writeExtents(ctx, ino, start, buf); err != nil {
		return err
	}

	file, node, err := tx.getFile(ctx, ino)
	if err != nil {
		return err
	}
	if end > file.Size() {
		file.SetSize(end)
		if err := tx.putNode(ctx, ino, node); err != nil {
			return err
		}
	}
	return nil
}

func (tx *FSTx) ReadAt(ctx context.Context, ino INode, offset int64, buf []byte) (int, error) {
	if offset < 0 {
		return 0, fmt.Errorf("offset cannot be negative")
	}
	if len(buf) == 0 {
		return 0, nil
	}

	var (
		overlaps   []extentEntry
		toRead     int64
		start, end uint64
	)
	if err := func() error {
		tx.mu.RLock()
		defer tx.mu.RUnlock()
		file, _, err := tx.getFile(ctx, ino)
		if err != nil {
			return err
		}
		if uint64(offset) >= file.Size() {
			return io.EOF
		}
		toRead = min(int64(len(buf)), int64(file.Size()-uint64(offset)))
		start = uint64(offset)
		end = start + uint64(toRead)
		overlaps, err = tx.getOverlappingExtents(ctx, ino, start, end)
		if err != nil {
			return err
		}
		return nil
	}(); err != nil {
		return 0, err
	}
	read := int(toRead)
	clear(buf[:read])
	for _, ee := range overlaps {
		extStart := ee.startAt()
		if err := tx.readExtent(ctx, ee.Ext, func(data []byte) error {
			if len(data) < int(ee.Ext.Len) {
				return fmt.Errorf("extent data is shorter than declared length: %d < %d", len(data), ee.Ext.Len)
			}
			copyStart := max(start, extStart)
			copyEnd := min(end, ee.EndAt)
			copy(buf[copyStart-start:copyEnd-start], data[copyStart-extStart:copyEnd-extStart])
			return nil
		}); err != nil {
			return 0, err
		}
	}
	if read < len(buf) {
		if read == 0 {
			return 0, io.EOF
		}
		return read, nil
	}
	return read, nil
}

type extentEntry struct {
	EndAt uint64
	Ext   Extent
}

func (ee *extentEntry) Unmarshal(ent gotkv.Entry) error {
	if len(ent.Key) != len(INode{})+8 {
		return fmt.Errorf("invalid extent key length: %d", len(ent.Key))
	}
	ee.EndAt = binary.BigEndian.Uint64(ent.Key[len(INode{}):])
	if err := ee.Ext.Unmarshal(ent.Value); err != nil {
		return err
	}
	if uint64(ee.Ext.Len) > ee.EndAt {
		return fmt.Errorf("invalid extent ending at %d with length %d", ee.EndAt, ee.Ext.Len)
	}
	return nil
}

func parseExtentEntry(ent gotkv.Entry) (extentEntry, error) {
	var ee extentEntry
	if err := ee.Unmarshal(ent); err != nil {
		return extentEntry{}, err
	}
	return ee, nil
}

func (ee extentEntry) startAt() uint64 {
	return ee.EndAt - uint64(ee.Ext.Len)
}

func isAllZero(x []byte) bool {
	for i := range x {
		if x[i] != 0 {
			return false
		}
	}
	return true
}
