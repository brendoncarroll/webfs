package webfs

import (
	"context"
	"fmt"
	"sync"

	"blobcache.io/blobcache/src/bcsdk"
	"blobcache.io/blobcache/src/blobcache"
	"github.com/gotvc/got/src/gdat"
	"github.com/gotvc/got/src/gotkv"
)

type VolumeConfig struct {
	NodeID   blobcache.NodeID `json:"node"`
	VolumeID blobcache.OID    `json:"volume"`
}

func (vc VolumeConfig) FQOID() blobcache.FQOID {
	return blobcache.FQOID{Node: vc.NodeID, OID: vc.VolumeID}
}

type machines struct {
	// fdata manages posting and getting file data
	fdata gdat.Machine
	// inodekv manages interactions with the inode table.
	inodekv gotkv.Machine
}

func newMachines(salt [32]byte, maxSize int) *machines {
	const (
		filedata = "filedata"
		inodekv  = "inodekv"
	)
	var dataSalt [32]byte
	gdat.DeriveKey(dataSalt[:], &salt, []byte(filedata))
	var inokvSalt [32]byte
	gdat.DeriveKey(inokvSalt[:], &salt, []byte(inodekv))
	return &machines{
		fdata: *gdat.NewMachine(gdat.Params{
			Salt: dataSalt,
		}),
		inodekv: gotkv.NewMachine(gotkv.Params{
			Salt:     inokvSalt,
			MaxSize:  maxSize,
			MeanSize: 1 << 13,
		}),
	}
}

// System manages a set of WebFS Volumes reachable from a single root Volume
type System struct {
	bc      blobcache.Service
	rootcfg VolumeConfig

	mu    sync.Mutex
	machs map[blobcache.FQOID]*machines
}

func NewSystem(svc blobcache.Service, rootcfg VolumeConfig) *System {
	return &System{
		rootcfg: rootcfg,
		bc:      svc,
	}
}

func (sys *System) Init(ctx context.Context) error {
	return nil
}

func (sys *System) ViewRoot(ctx context.Context, fn func(*Tx) error) error {
	volh, err := sys.bc.OpenFiat(ctx, sys.rootcfg.VolumeID, blobcache.Action_ALL)
	if err != nil {
		return err
	}
	txn, err := bcsdk.BeginTx(ctx, sys.bc, *volh, blobcache.TxParams{})
	if err != nil {
		return err
	}
	defer txn.Abort(ctx)
	ep, err := sys.bc.Endpoint(ctx)
	if err != nil {
		return err
	}
	fqoid := blobcache.FQOID{OID: volh.OID, Node: ep.Node}
	txn2, err := sys.wrapTx(ctx, txn, fqoid)
	if err != nil {
		return err
	}
	return fn(txn2)
}

func (sys *System) ModifyRoot(ctx context.Context, fn func(*Tx) error) error {
	volh, err := sys.bc.OpenFiat(ctx, sys.rootcfg.VolumeID, blobcache.Action_ALL)
	if err != nil {
		return err
	}
	txn, err := bcsdk.BeginTx(ctx, sys.bc, *volh, blobcache.TxParams{Modify: true})
	if err != nil {
		return err
	}
	defer txn.Abort(ctx)
	ep, err := sys.bc.Endpoint(ctx)
	if err != nil {
		return err
	}
	fqoid := blobcache.FQOID{OID: volh.OID, Node: ep.Node}
	txn2, err := sys.wrapTx(ctx, txn, fqoid)
	if err != nil {
		return err
	}
	if err := fn(txn2); err != nil {
		return err
	}
	newRoot, err := txn2.Flush(ctx)
	if err != nil {
		return err
	}
	if err := txn.Save(ctx, newRoot.Marshal(nil)); err != nil {
		return err
	}
	return txn.Commit(ctx)
}

func (sys *System) getMachs(fqoid blobcache.FQOID, salt [32]byte, maxSize int) *machines {
	sys.mu.Lock()
	defer sys.mu.Unlock()
	if sys.machs == nil {
		sys.machs = make(map[blobcache.FQOID]*machines)
	}
	if machs, exists := sys.machs[fqoid]; exists {
		return machs
	}
	machs := newMachines(salt, maxSize)
	sys.machs[fqoid] = machs
	return machs
}

func (sys *System) wrapTx(ctx context.Context, txn *bcsdk.Tx, fqoid blobcache.FQOID) (*Tx, error) {
	root, err := LoadRoot(ctx, txn)
	if err != nil {
		return nil, err
	}
	machs := sys.getMachs(fqoid, [32]byte{}, min(int(root.maxSize), txn.MaxSize()))
	return &Tx{
		root: root,
		ros:  txn,
		rws:  txn,
		link: func(ctx context.Context, target blobcache.OID) (blobcache.LinkToken, error) {
			return blobcache.LinkToken{}, nil
		},
		machs:   machs,
		inodetx: machs.inodekv.NewTx(txn, root.inodes),
	}, nil
}

// Tx is a transaction on a webfs volume.
type Tx struct {
	root Root
	ros  bcsdk.RO
	rws  bcsdk.RW
	link func(ctx context.Context, target blobcache.OID) (blobcache.LinkToken, error)

	machs   *machines
	inodetx *gotkv.Tx

	dirCache map[]Dir
		fileCache map[]File
}

// Flush writes out the changes to the store and returns a new root.
func (tx *Tx) Flush(ctx context.Context) (Root, error) {
	inodekvroot, err := tx.inodetx.Flush(ctx)
	if err != nil {
		return Root{}, err
	}
	return Root{inodes: inodekvroot}, nil
}

func (tx *Tx) GetNode(ctx context.Context, ino INode) (Node, error) {
	var val []byte
	if exists, err := tx.inodetx.Get(ctx, ino[:], &val); err != nil {
		return Node{}, err
	} else if !exists {
		return Node{}, fmt.Errorf("inode (%v) does not exist ", ino)
	}
	var ret Node
	return ret, ret.Unmarshal(val)
}

func (tx *Tx) putNode(ctx context.Context, ino INode, node Node) error {

}
