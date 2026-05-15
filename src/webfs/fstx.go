package webfs

import (
	"context"
	"fmt"
	"sync"

	"blobcache.io/blobcache/src/bcsdk"
	"blobcache.io/blobcache/src/blobcache"
	capnp "capnproto.org/go/capnp/v3"
	"github.com/brendoncarroll/webfs/src/internal/gdatcache"
	"github.com/brendoncarroll/webfs/src/internal/wfscnp"
	"github.com/gotvc/got/src/gdat"
	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/tai64"
	"go.inet256.org/inet256/src/inet256"
)

type Linker interface {
	Link(ctx context.Context, target blobcache.Handle, mask blobcache.ActionSet) (*blobcache.LinkToken, error)
	Unlink(ctx context.Context, targets []blobcache.LinkTokenID) error
}

// FSTx is a transaction on a single webfs volume's filesystem state.
type FSTx struct {
	// prev is the previous existing state, without any pending changes
	prev      FSState
	ros       bcsdk.RO
	rws       bcsdk.RW
	volLinker Linker
	gid       GID
	pki       *inet256.PKI
	priv      inet256.PrivateKey
	fdata     *gdat.Machine
	pkcache   *gdatcache.Cache[inet256.PublicKey]

	// mu guards all the gotkvTx's
	mu         sync.RWMutex
	inodetx    *gotkv.Tx
	xattrtx    *gotkv.Tx
	sessiontx  *gotkv.Tx
	locktx     *gotkv.Tx
	inodeCache map[INode]wfscnp.Node
}

func newFSTx(prev FSState, s bcsdk.RW, link Linker, machs *machines, pki *inet256.PKI, priv inet256.PrivateKey) *FSTx {
	return &FSTx{
		prev:      prev,
		ros:       s,
		rws:       s,
		volLinker: link,
		gid:       prev.gid,
		pki:       pki,
		priv:      priv,

		fdata:     &machs.fdata,
		pkcache:   newPublicKeyCache(&machs.fdata, pki, 16),
		inodetx:   machs.inodekv.NewTx(s, prev.inodes),
		xattrtx:   machs.xattrkv.NewTx(s, prev.xattrs),
		sessiontx: machs.sessionkv.NewTx(s, prev.sessions),
		locktx:    machs.lockkv.NewTx(s, prev.locks),
	}
}

// Flush writes out the changes to the store and returns a new root.
func (tx *FSTx) Flush(ctx context.Context) (FSState, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	inodekvroot, err := tx.inodetx.Flush(ctx)
	if err != nil {
		return FSState{}, err
	}
	xattrkvroot, err := tx.xattrtx.Flush(ctx)
	if err != nil {
		return FSState{}, err
	}
	sessionkvroot, err := tx.sessiontx.Flush(ctx)
	if err != nil {
		return FSState{}, err
	}
	lockkvroot, err := tx.locktx.Flush(ctx)
	if err != nil {
		return FSState{}, err
	}
	tx.prev.inodes = inodekvroot
	tx.prev.xattrs = xattrkvroot
	tx.prev.sessions = sessionkvroot
	tx.prev.locks = lockkvroot
	return tx.prev, nil
}

// getNode assumes the lock is held
func (tx *FSTx) getNode(ctx context.Context, ino INode) (wfscnp.Node, error) {
	if tx.inodeCache != nil {
		if cached, exists := tx.inodeCache[ino]; exists {
			return cached, nil
		}
	}

	var val []byte
	if exists, err := tx.inodetx.Get(ctx, ino[:], &val); err != nil {
		return wfscnp.Node{}, err
	} else if !exists {
		return wfscnp.Node{}, fmt.Errorf("inode (%v) does not exist ", ino)
	}
	msg, err := capnp.Unmarshal(val)
	if err != nil {
		return wfscnp.Node{}, err
	}
	ret, err := wfscnp.ReadRootNode(msg)
	if err != nil {
		return wfscnp.Node{}, err
	}
	if tx.inodeCache == nil {
		tx.inodeCache = make(map[INode]wfscnp.Node)
	}
	tx.inodeCache[ino] = ret
	return ret, nil
}

// putNode overwrites a node.  It requires the lock.
func (tx *FSTx) putNode(ctx context.Context, ino INode, node wfscnp.Node) error {
	msg := node.Message()
	if msg == nil {
		return fmt.Errorf("cannot write invalid node for inode (%v)", ino)
	}
	data, err := msg.Marshal()
	if err != nil {
		return err
	}
	if err := tx.inodetx.Put(ctx, ino[:], data); err != nil {
		return err
	}
	if tx.inodeCache == nil {
		tx.inodeCache = make(map[INode]wfscnp.Node)
	}
	tx.inodeCache[ino] = node
	return nil
}

// setRoot acquires the lock and then calls putNode on the root inode
func (tx *FSTx) setRoot(ctx context.Context, node wfscnp.Node) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	return tx.putNode(ctx, INode{}, node)
}

func (tx *FSTx) StatINode(ctx context.Context, ino INode) (INodeStats, error) {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	node, err := tx.getNode(ctx, ino)
	if err != nil {
		return INodeStats{}, err
	}
	return INodeStats{RefCount: node.RefCount()}, nil
}

func (tx *FSTx) GetModifiedAt(ctx context.Context, ino INode) (tai64.TAI64N, error) {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	node, err := tx.getNode(ctx, ino)
	if err != nil {
		return tai64.TAI64N{}, err
	}
	ts, err := node.ModifiedAt()
	if err != nil {
		return tai64.TAI64N{}, err
	}
	return tai64.TAI64N{Seconds: ts.Seconds(), Nanoseconds: ts.Nanoseconds()}, nil
}

func (tx *FSTx) SetModifiedAt(ctx context.Context, ino INode, t tai64.TAI64N) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	node, err := tx.getNode(ctx, ino)
	if err != nil {
		return err
	}
	mt, err := node.ModifiedAt()
	if err != nil || !node.HasModifiedAt() {
		mt, err = node.NewModifiedAt()
		if err != nil {
			return err
		}
	}
	mt.SetSeconds(t.Seconds)
	mt.SetNanoseconds(t.Nanoseconds)
	return tx.putNode(ctx, ino, node)
}
