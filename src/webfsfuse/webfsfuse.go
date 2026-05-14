package webfsfuse

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	iofs "io/fs"
	"sync"
	"syscall"
	"time"

	"github.com/brendoncarroll/webfs/src/webfs"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.brendoncarroll.net/tai64"
	"go.inet256.org/inet256/src/inet256"
	"golang.org/x/sys/unix"
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
var _ = (fs.NodeLinker)((*Node)(nil))
var _ = (fs.NodeRenamer)((*Node)(nil))
var _ = (fs.NodeSymlinker)((*Node)(nil))
var _ = (fs.NodeReadlinker)((*Node)(nil))
var _ = (fs.NodeAllocater)((*Node)(nil))
var _ = (fs.NodeLseeker)((*Node)(nil))
var _ = (fs.NodeUnlinker)((*Node)(nil))
var _ = (fs.NodeRmdirer)((*Node)(nil))
var _ = (fs.NodeGetxattrer)((*Node)(nil))
var _ = (fs.NodeSetxattrer)((*Node)(nil))
var _ = (fs.NodeRemovexattrer)((*Node)(nil))
var _ = (fs.NodeListxattrer)((*Node)(nil))

type fileHandle struct {
	node   *Node
	append bool
	mu     sync.Mutex
	locks  map[uint64]struct{}
	closed bool
}

var _ = (fs.FileReleaser)((*fileHandle)(nil))
var _ = (fs.FileGetlker)((*fileHandle)(nil))
var _ = (fs.FileSetlker)((*fileHandle)(nil))
var _ = (fs.FileSetlkwer)((*fileHandle)(nil))

// FS is a FileSystem
type FS struct {
	sys     *webfs.System
	pki     inet256.PKI
	rootCfg webfs.VolumeConfig
	root    Node
}

func New(sys *webfs.System, pki inet256.PKI, rootCfg webfs.VolumeConfig) FS {
	return FS{
		sys:     sys,
		pki:     pki,
		rootCfg: rootCfg,
	}
}

func (fsys *FS) Root() *Node {
	fsys.root.fsys = fsys
	return &fsys.root
}

type Node struct {
	fs.Inode
	fsys *FS
	ino  webfs.INode

	mu   sync.RWMutex
	size uint64
}

func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	var ent webfs.DirEnt
	err := n.fsys.sys.View(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		var err error
		ent, err = tx.GetChild(ctx, n.ino, name)
		return err
	})
	if err != nil {
		return nil, toErrno(err)
	}
	return n.newChild(ctx, ent.Target, out)
}

func (n *Node) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	mode, sz, links, mtime, errno := n.refreshStats(ctx)
	if errno != 0 {
		return errno
	}
	out.Mode = fuseMode(mode)
	out.Size = sz
	out.Nlink = links
	out.SetTimes(nil, &mtime, nil)
	return 0
}

func (n *Node) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	ents := make([]fuse.DirEntry, 0, 16)
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		for ent, err := range tx.ReadDir(ctx, n.ino, "") {
			if err != nil {
				return err
			}
			mode, err := tx.GetMode(ctx, ent.Target)
			if err != nil {
				return err
			}
			ents = append(ents, fuse.DirEntry{
				Name: ent.Name,
				Mode: fuseMode(mode),
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
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		var err error
		ino, err = tx.CreateDirAt(ctx, n.ino, name, childMode)
		return err
	})
	if err != nil {
		return nil, toErrno(err)
	}
	return n.newChild(ctx, ino, out)
}

func (n *Node) Create(ctx context.Context, name string, _ uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	childMode := iofs.FileMode(mode & 0o7777)
	var ino webfs.INode
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		var err error
		ino, err = tx.CreateFileAt(ctx, n.ino, name, childMode, webfs.FileParams{Now: tai64.Now(), BlockSize: 4096})
		return err
	})
	if err != nil {
		return nil, nil, 0, toErrno(err)
	}
	ch, errno := n.newChild(ctx, ino, out)
	if errno != 0 {
		return nil, nil, 0, errno
	}
	return ch, newFileHandle(ch.Operations().(*Node), false), 0, 0
}

func (n *Node) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	mode, _, _, _, errno := n.refreshStats(ctx)
	if errno != 0 {
		return nil, 0, errno
	}
	if mode.IsDir() || mode&iofs.ModeSymlink != 0 {
		return nil, 0, syscall.EISDIR
	}
	return newFileHandle(n, flags&syscall.O_APPEND != 0), 0, 0
}

func (n *Node) Read(ctx context.Context, _ fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	var nRead int
	err := n.fsys.sys.View(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
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
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
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
	if modeBits, ok := in.GetMode(); ok {
		mode, _, _, _, errno := n.refreshStats(ctx)
		if errno != 0 {
			return errno
		}
		newMode := (mode &^ 0o7777) | iofs.FileMode(modeBits&0o7777)
		err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
			return tx.SetMode(ctx, n.ino, newMode)
		})
		if err != nil {
			return toErrno(err)
		}
	}
	if sz, ok := in.GetSize(); ok {
		err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
			return tx.Truncate(ctx, n.ino, sz)
		})
		if err != nil {
			return toErrno(err)
		}
		n.setSize(sz)
	}
	if mtime, ok := in.GetMTime(); ok {
		err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
			return tx.SetModifiedAt(ctx, n.ino, tai64.FromGoTime(mtime))
		})
		if err != nil {
			return toErrno(err)
		}
	}
	mode, sz, links, mtime, errno := n.refreshStats(ctx)
	if errno != 0 {
		return errno
	}
	out.Mode = fuseMode(mode)
	out.Size = sz
	out.Nlink = links
	out.SetTimes(nil, &mtime, nil)
	return 0
}

func (n *Node) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	targetNode, ok := target.(*Node)
	if !ok {
		if op, ok := target.EmbeddedInode().Operations().(*Node); ok {
			targetNode = op
		} else {
			return nil, syscall.EINVAL
		}
	}
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		return tx.Link(ctx, n.ino, name, targetNode.ino)
	})
	if err != nil {
		return nil, toErrno(err)
	}
	mode, sz, links, _, errno := targetNode.refreshStats(ctx)
	if errno != 0 {
		return nil, errno
	}
	out.Mode = fuseMode(mode)
	out.Size = sz
	out.Nlink = links
	return targetNode.EmbeddedInode(), 0
}

func (n *Node) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	if flags != 0 {
		return syscall.ENOTSUP
	}
	np, ok := newParent.(*Node)
	if !ok {
		if op, ok := newParent.EmbeddedInode().Operations().(*Node); ok {
			np = op
		} else {
			return syscall.EINVAL
		}
	}
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		return tx.Rename(ctx, n.ino, name, np.ino, newName)
	})
	if err != nil {
		return toErrno(err)
	}
	return 0
}

func (n *Node) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	var ino webfs.INode
	mode := iofs.ModeSymlink | 0o777
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		var err error
		ino, err = tx.CreateFileAt(ctx, n.ino, name, mode, webfs.FileParams{Now: tai64.Now(), BlockSize: 4096})
		if err != nil {
			return err
		}
		return tx.WriteAt(ctx, ino, 0, []byte(target))
	})
	if err != nil {
		return nil, toErrno(err)
	}
	child, errno := n.newChild(ctx, ino, out)
	if errno != 0 {
		return nil, errno
	}
	return child, 0
}

func (n *Node) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	mode, _, _, _, errno := n.refreshStats(ctx)
	if errno != 0 {
		return nil, errno
	}
	if mode&iofs.ModeSymlink == 0 {
		return nil, syscall.EINVAL
	}
	var out []byte
	err := n.fsys.sys.View(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		st, err := tx.StatFile(ctx, n.ino)
		if err != nil {
			return err
		}
		buf := make([]byte, st.Size)
		_, err = tx.ReadAt(ctx, n.ino, 0, buf)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		if err != nil {
			return err
		}
		out = buf
		return nil
	})
	if err != nil {
		return nil, toErrno(err)
	}
	return out, 0
}

func (n *Node) Allocate(ctx context.Context, _ fs.FileHandle, off uint64, size uint64, mode uint32) syscall.Errno {
	if mode&unix.FALLOC_FL_KEEP_SIZE != 0 {
		return 0
	}
	end := off + size
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		st, err := tx.StatFile(ctx, n.ino)
		if err != nil {
			return err
		}
		if end <= st.Size {
			return nil
		}
		return tx.Truncate(ctx, n.ino, end)
	})
	if err != nil {
		return toErrno(err)
	}
	n.setSize(end)
	return 0
}

func (n *Node) Lseek(ctx context.Context, _ fs.FileHandle, off uint64, whence uint32) (uint64, syscall.Errno) {
	var sz uint64
	err := n.fsys.sys.View(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		st, err := tx.StatFile(ctx, n.ino)
		if err != nil {
			return err
		}
		sz = st.Size
		return nil
	})
	if err != nil {
		return 0, toErrno(err)
	}
	switch whence {
	case unix.SEEK_DATA:
		if off >= sz {
			return 0, syscall.ENXIO
		}
		return off, 0
	case unix.SEEK_HOLE:
		if off > sz {
			return 0, syscall.ENXIO
		}
		return sz, 0
	default:
		return 0, syscall.EINVAL
	}
}

func (n *Node) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	var value []byte
	err := n.fsys.sys.View(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		var err error
		value, err = tx.GetXAttr(ctx, n.ino, webfs.XAttrKey(attr))
		return err
	})
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return 0, fs.ENOATTR
		}
		return 0, toErrno(err)
	}
	size := uint32(len(value))
	if len(dest) == 0 {
		return size, 0
	}
	if len(dest) < len(value) {
		return size, syscall.ERANGE
	}
	copy(dest, value)
	return size, 0
}

func (n *Node) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	if flags&(unix.XATTR_CREATE|unix.XATTR_REPLACE) == (unix.XATTR_CREATE | unix.XATTR_REPLACE) {
		return syscall.EINVAL
	}
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		_, err := tx.GetXAttr(ctx, n.ino, webfs.XAttrKey(attr))
		exists := err == nil
		if err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return err
		}
		if flags&unix.XATTR_CREATE != 0 && exists {
			return iofs.ErrExist
		}
		if flags&unix.XATTR_REPLACE != 0 && !exists {
			return iofs.ErrNotExist
		}
		return tx.SetXAttr(ctx, n.ino, webfs.XAttrKey(attr), data)
	})
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return fs.ENOATTR
		}
		return toErrno(err)
	}
	return 0
}

func (n *Node) Removexattr(ctx context.Context, attr string) syscall.Errno {
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		if _, err := tx.GetXAttr(ctx, n.ino, webfs.XAttrKey(attr)); err != nil {
			return err
		}
		return tx.RemoveXAttr(ctx, n.ino, webfs.XAttrKey(attr))
	})
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return fs.ENOATTR
		}
		return toErrno(err)
	}
	return 0
}

func (n *Node) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	var keys []webfs.XAttrKey
	err := n.fsys.sys.View(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		var err error
		keys, err = tx.ListXAttrs(ctx, n.ino)
		return err
	})
	if err != nil {
		return 0, toErrno(err)
	}
	var size uint32
	for _, key := range keys {
		size += uint32(len(key) + 1)
	}
	if len(dest) == 0 {
		return size, 0
	}
	if len(dest) < int(size) {
		return size, syscall.ERANGE
	}
	off := 0
	for _, key := range keys {
		off += copy(dest[off:], string(key))
		dest[off] = 0
		off++
	}
	return size, 0
}

func (f *fileHandle) Getlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) syscall.Errno {
	if (flags & fuse.FUSE_LK_FLOCK) != 0 {
		return syscall.ENOTSUP
	}
	start, length, errno := fileLockRange(lk)
	if errno != 0 {
		return errno
	}
	kind := webfs.LockKindWrite
	if lk.Typ == syscall.F_RDLCK {
		kind = webfs.LockKindRead
	}
	out.Start = lk.Start
	out.End = lk.End
	out.Typ = syscall.F_UNLCK
	out.Pid = 0
	sessionID, err := f.node.fsys.lockSessionID()
	if err != nil {
		return toErrno(err)
	}
	err = f.node.fsys.sys.View(ctx, f.node.fsys.rootCfg, func(tx *webfs.Tx) error {
		conflict, err := tx.FindConflictingLock(ctx, f.node.ino, sessionID, owner, kind, start, length)
		if err != nil {
			return err
		}
		if conflict == nil {
			return nil
		}
		out.Start = conflict.State.Start()
		out.End = webfsLockEnd(conflict.State.Start(), conflict.State.Length())
		out.Typ = fuseLockTypeFromWebfs(webfs.LockKind(conflict.State.Kind()))
		out.Pid = 0
		return nil
	})
	if err != nil {
		return toErrno(err)
	}
	return 0
}

func (f *fileHandle) Setlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	return f.setlk(ctx, owner, lk, flags)
}

func (f *fileHandle) Setlkw(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	return f.setlk(ctx, owner, lk, flags)
}

func (f *fileHandle) setlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	if (flags & fuse.FUSE_LK_FLOCK) != 0 {
		return syscall.ENOTSUP
	}
	kind, start, length, errno := fileLockToWebfs(lk)
	if errno != 0 {
		return errno
	}
	var sessionID inet256.ID
	err := f.node.fsys.sys.Modify(ctx, f.node.fsys.rootCfg, func(tx *webfs.Tx) error {
		var err error
		sessionID, err = tx.EnsureSession(ctx, tai64.Now())
		if err != nil {
			return err
		}
		return tx.SetLock(ctx, sessionID, owner, f.node.ino, kind, start, length)
	})
	if err != nil {
		if errors.Is(err, webfs.ErrLockConflict) {
			return syscall.EAGAIN
		}
		return toErrno(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if kind == 0 {
		delete(f.locks, owner)
	} else {
		if f.locks == nil {
			f.locks = make(map[uint64]struct{})
		}
		f.locks[owner] = struct{}{}
	}
	return 0
}

func (f *fileHandle) Release(ctx context.Context) syscall.Errno {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return 0
	}
	f.closed = true
	owners := make([]uint64, 0, len(f.locks))
	for owner := range f.locks {
		owners = append(owners, owner)
	}
	f.mu.Unlock()
	if len(owners) == 0 {
		return 0
	}
	err := f.node.fsys.sys.Modify(ctx, f.node.fsys.rootCfg, func(tx *webfs.Tx) error {
		sessionID, err := tx.EnsureSession(ctx, tai64.Now())
		if err != nil {
			return err
		}
		for _, owner := range owners {
			if err := tx.SetLock(ctx, sessionID, owner, f.node.ino, 0, 0, 0); err != nil && !errors.Is(err, iofs.ErrNotExist) {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return toErrno(err)
	}
	return 0
}

func newFileHandle(n *Node, appendMode bool) *fileHandle {
	return &fileHandle{node: n, append: appendMode}
}

func (fsys *FS) lockSessionID() (inet256.ID, error) {
	data, err := hex.DecodeString(fsys.rootCfg.PrivateKeyHex)
	if err != nil {
		return inet256.ID{}, err
	}
	privKey, err := fsys.pki.ParsePrivateKey(data)
	if err != nil {
		return inet256.ID{}, err
	}
	return fsys.pki.NewID(inet256.PublicFromPrivate(privKey)), nil
}

func (n *Node) Unlink(ctx context.Context, name string) syscall.Errno {
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		ent, err := tx.GetChild(ctx, n.ino, name)
		if err != nil {
			return err
		}
		mode, err := tx.GetMode(ctx, ent.Target)
		if err != nil {
			return err
		}
		if mode.IsDir() {
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
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		ent, err := tx.GetChild(ctx, n.ino, name)
		if err != nil {
			return err
		}
		mode, err := tx.GetMode(ctx, ent.Target)
		if err != nil {
			return err
		}
		if !mode.IsDir() {
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

func (n *Node) newChild(ctx context.Context, ino webfs.INode, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	op := &Node{
		fsys: n.fsys,
		ino:  ino,
	}
	mode, sz, links, _, errno := op.refreshStats(ctx)
	if errno != 0 {
		return nil, errno
	}
	out.Nlink = links
	out.Mode = fuseMode(mode)
	out.Size = sz
	stable := fs.StableAttr{Ino: fuseIno(ino), Mode: out.Mode & syscall.S_IFMT}
	return n.NewInode(ctx, op, stable), 0
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
	if mode&iofs.ModeSymlink != 0 {
		return syscall.S_IFLNK | perm
	}
	if mode.IsDir() {
		return syscall.S_IFDIR | perm
	}
	return syscall.S_IFREG | perm
}

func (n *Node) refreshStats(ctx context.Context) (iofs.FileMode, uint64, uint32, time.Time, syscall.Errno) {
	var mode iofs.FileMode
	var size uint64
	var links uint32 = 1
	var mtime time.Time
	err := n.fsys.sys.View(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		m, err := tx.GetMode(ctx, n.ino)
		if err != nil {
			return err
		}
		mode = m
		taiMtime, err := tx.GetModifiedAt(ctx, n.ino)
		if err != nil {
			return err
		}
		mtime = taiMtime.GoTime()
		st, err := tx.StatINode(ctx, n.ino)
		if err != nil {
			return err
		}
		links = st.RefCount
		if !mode.IsDir() || mode&iofs.ModeSymlink != 0 {
			f, err := tx.StatFile(ctx, n.ino)
			if err != nil {
				return err
			}
			size = f.Size
		}
		return nil
	})
	if err != nil {
		return 0, 0, 0, time.Time{}, toErrno(err)
	}
	if links == 0 {
		links = 1
	}
	n.setSize(size)
	return mode, size, links, mtime, 0
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

func fileLockToWebfs(lk *fuse.FileLock) (webfs.LockKind, uint64, uint64, syscall.Errno) {
	start, length, errno := fileLockRange(lk)
	if errno != 0 {
		return 0, 0, 0, errno
	}
	kind, errno := fuseLockTypeToWebfs(lk.Typ)
	if errno != 0 {
		return 0, 0, 0, errno
	}
	return kind, start, length, 0
}

func fileLockRange(lk *fuse.FileLock) (uint64, uint64, syscall.Errno) {
	start := lk.Start
	if lk.End == ^uint64(0) || lk.End == (1<<63)-1 {
		return start, 0, 0
	}
	if lk.End < start {
		return 0, 0, syscall.EINVAL
	}
	return start, lk.End - start + 1, 0
}

func fuseLockTypeToWebfs(typ uint32) (webfs.LockKind, syscall.Errno) {
	switch typ {
	case syscall.F_RDLCK:
		return webfs.LockKindRead, 0
	case syscall.F_WRLCK:
		return webfs.LockKindWrite, 0
	case syscall.F_UNLCK:
		return 0, 0
	default:
		return 0, syscall.EINVAL
	}
}

func (n *Node) lockSessionID(ctx context.Context) (inet256.ID, error) {
	var sessionID inet256.ID
	err := n.fsys.sys.Modify(ctx, n.fsys.rootCfg, func(tx *webfs.Tx) error {
		id, err := tx.EnsureSession(ctx, tai64.Now())
		if err != nil {
			return err
		}
		sessionID = id
		return nil
	})
	return sessionID, err
}

func fuseLockTypeFromWebfs(kind webfs.LockKind) uint32 {
	switch kind {
	case webfs.LockKindRead:
		return syscall.F_RDLCK
	case webfs.LockKindWrite:
		return syscall.F_WRLCK
	default:
		return syscall.F_UNLCK
	}
}

func webfsLockEnd(start, length uint64) uint64 {
	if length == 0 {
		return (1 << 63) - 1
	}
	return start + length - 1
}
