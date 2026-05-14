package webfsfuse

import (
	"context"
	"errors"
	"io"
	iofs "io/fs"
	"sync"
	"syscall"

	"github.com/brendoncarroll/webfs/src/webfs"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.brendoncarroll.net/tai64"
)

// Node types must be InodeEmbedders
var _ = (fs.InodeEmbedder)((*Node)(nil))

// Node types should implement some file system operations, eg. Lookup
var _ = (fs.NodeLookuper)((*Node)(nil))
var _ = (fs.NodeGetattrer)((*Node)(nil))
var _ = (fs.NodeReaddirer)((*Node)(nil))
var _ = (fs.NodeMkdirer)((*Node)(nil))
var _ = (fs.NodeCreater)((*Node)(nil))
var _ = (fs.NodeOpener)((*Node)(nil))
var _ = (fs.NodeReader)((*Node)(nil))
var _ = (fs.NodeWriter)((*Node)(nil))
var _ = (fs.NodeSetattrer)((*Node)(nil))
var _ = (fs.NodeUnlinker)((*Node)(nil))
var _ = (fs.NodeRmdirer)((*Node)(nil))

type fileHandle struct {
	append bool
}

type Node struct {
	fs.Inode
	sys     *webfs.System
	rootCfg webfs.VolumeConfig
	ino     webfs.INode
	mode    iofs.FileMode

	mu   sync.RWMutex
	size uint64
}

func NewRoot(sys *webfs.System, rootCfg webfs.VolumeConfig) *Node {
	return &Node{
		sys:     sys,
		rootCfg: rootCfg,
		ino:     webfs.INode{},
		mode:    iofs.ModeDir | 0o755,
	}
}

func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	var ent webfs.DirEnt
	err := n.sys.View(ctx, n.rootCfg, func(tx *webfs.Tx) error {
		var err error
		ent, err = tx.GetChild(ctx, n.ino, name)
		return err
	})
	if err != nil {
		return nil, toErrno(err)
	}
	return n.newChild(ctx, ent.Target, ent.Mode, out)
}

func (n *Node) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = fuseMode(n.mode)
	if !n.mode.IsDir() {
		out.Size = n.getSize()
	}
	return 0
}

func (n *Node) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	ents := make([]fuse.DirEntry, 0, 16)
	err := n.sys.Modify(ctx, n.rootCfg, func(tx *webfs.Tx) error {
		for ent, err := range tx.ReadDir(ctx, n.ino, "") {
			if err != nil {
				return err
			}
			ents = append(ents, fuse.DirEntry{
				Name: ent.Name,
				Mode: fuseMode(ent.Mode),
			})
		}
		return nil
	})
	if err != nil {
		return nil, toErrno(err)
	}
	return fs.NewListDirStream(ents), 0
}

func (n *Node) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	childMode := iofs.ModeDir | iofs.FileMode(mode&0o7777)
	var ino webfs.INode
	err := n.sys.Modify(ctx, n.rootCfg, func(tx *webfs.Tx) error {
		var err error
		ino, err = tx.CreateDirAt(ctx, n.ino, name)
		return err
	})
	if err != nil {
		return nil, toErrno(err)
	}
	return n.newChild(ctx, ino, childMode, out)
}

func (n *Node) Create(ctx context.Context, name string, _ uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	childMode := iofs.FileMode(mode & 0o7777)
	var ino webfs.INode
	err := n.sys.Modify(ctx, n.rootCfg, func(tx *webfs.Tx) error {
		var err error
		ino, err = tx.CreateFileAt(ctx, n.ino, name, childMode, webfs.FileParams{Now: tai64.Now(), BlockSize: 4096})
		return err
	})
	if err != nil {
		return nil, nil, 0, toErrno(err)
	}
	ch, errno := n.newChild(ctx, ino, childMode, out)
	if errno != 0 {
		return nil, nil, 0, errno
	}
	return ch, nil, 0, 0
}

func (n *Node) Open(_ context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if n.mode.IsDir() {
		return nil, 0, syscall.EISDIR
	}
	return &fileHandle{append: flags&syscall.O_APPEND != 0}, 0, 0
}

func (n *Node) Read(ctx context.Context, _ fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	var nRead int
	err := n.sys.View(ctx, n.rootCfg, func(tx *webfs.Tx) error {
		read, err := tx.ReadAt(ctx, n.ino, off, dest)
		nRead = read
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	})
	if err != nil {
		return nil, toErrno(err)
	}
	return fuse.ReadResultData(dest[:nRead]), 0
}

func (n *Node) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	appendMode := false
	if h, ok := fh.(*fileHandle); ok {
		appendMode = h.append
	}
	err := n.sys.Modify(ctx, n.rootCfg, func(tx *webfs.Tx) error {
		if appendMode {
			st, err := tx.StatFile(ctx, n.ino)
			if err != nil {
				return err
			}
			off = int64(st.Size)
		}
		return tx.WriteAt(ctx, n.ino, off, data)
	})
	if err != nil {
		return 0, toErrno(err)
	}
	end := uint64(off) + uint64(len(data))
	if end > n.getSize() {
		n.setSize(end)
	}
	return uint32(len(data)), 0
}

func (n *Node) Setattr(ctx context.Context, _ fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if sz, ok := in.GetSize(); ok {
		err := n.sys.Modify(ctx, n.rootCfg, func(tx *webfs.Tx) error {
			return tx.Truncate(ctx, n.ino, sz)
		})
		if err != nil {
			return toErrno(err)
		}
		n.setSize(sz)
	}
	out.Mode = fuseMode(n.mode)
	out.Size = n.getSize()
	return 0
}

func (n *Node) Unlink(ctx context.Context, name string) syscall.Errno {
	err := n.sys.Modify(ctx, n.rootCfg, func(tx *webfs.Tx) error {
		ent, err := tx.GetChild(ctx, n.ino, name)
		if err != nil {
			return err
		}
		if ent.Mode.IsDir() {
			return syscall.EISDIR
		}
		return tx.Unlink(ctx, n.ino, name)
	})
	if err != nil {
		return toErrno(err)
	}
	return 0
}

func (n *Node) Rmdir(ctx context.Context, name string) syscall.Errno {
	err := n.sys.Modify(ctx, n.rootCfg, func(tx *webfs.Tx) error {
		ent, err := tx.GetChild(ctx, n.ino, name)
		if err != nil {
			return err
		}
		if !ent.Mode.IsDir() {
			return syscall.ENOTDIR
		}
		for _, err := range tx.ReadDir(ctx, ent.Target, "") {
			if err != nil {
				return err
			}
			return syscall.ENOTEMPTY
		}
		return tx.Unlink(ctx, n.ino, name)
	})
	if err != nil {
		return toErrno(err)
	}
	return 0
}

func (n *Node) newChild(ctx context.Context, ino webfs.INode, mode iofs.FileMode, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	op := &Node{
		sys:     n.sys,
		rootCfg: n.rootCfg,
		ino:     ino,
		mode:    mode,
	}
	if !mode.IsDir() {
		sz, err := op.statFileSize(ctx)
		if err != nil {
			return nil, toErrno(err)
		}
		op.setSize(sz)
	}
	out.Mode = fuseMode(mode)
	out.Size = op.getSize()
	stable := fs.StableAttr{Ino: fuseIno(ino), Mode: out.Mode & syscall.S_IFMT}
	return n.NewInode(ctx, op, stable), 0
}

func (n *Node) statFileSize(ctx context.Context) (uint64, error) {
	var size uint64
	err := n.sys.View(ctx, n.rootCfg, func(tx *webfs.Tx) error {
		st, err := tx.StatFile(ctx, n.ino)
		if err != nil {
			return err
		}
		size = st.Size
		return nil
	})
	return size, err
}

func (n *Node) getSize() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.size
}

func (n *Node) setSize(sz uint64) {
	n.mu.Lock()
	n.size = sz
	n.mu.Unlock()
}

func fuseIno(ino webfs.INode) uint64 {
	ret := uint64(1469598103934665603)
	for i := range ino {
		ret ^= uint64(ino[i])
		ret *= 1099511628211
	}
	if ret == 0 {
		return 1
	}
	return ret
}

func fuseMode(mode iofs.FileMode) uint32 {
	perm := uint32(mode.Perm())
	if mode.IsDir() {
		return syscall.S_IFDIR | perm
	}
	return syscall.S_IFREG | perm
}

func toErrno(err error) syscall.Errno {
	if err == nil {
		return 0
	}
	if errno, ok := err.(syscall.Errno); ok {
		return errno
	}
	if errors.Is(err, iofs.ErrNotExist) {
		return syscall.ENOENT
	}
	if errors.Is(err, iofs.ErrExist) {
		return syscall.EEXIST
	}
	if errors.Is(err, iofs.ErrInvalid) {
		return syscall.EINVAL
	}
	return syscall.EIO
}
