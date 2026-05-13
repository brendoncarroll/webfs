package webfs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"blobcache.io/blobcache/src/bcsdk"
	"blobcache.io/blobcache/src/blobcache"
	capnp "capnproto.org/go/capnp/v3"
	"github.com/brendoncarroll/webfs/src/internal/wfscnp"
	"github.com/cloudflare/circl/sign"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/gotvc/got/src/gdat"
	"github.com/gotvc/got/src/gotkv"
	"go.inet256.org/inet256/src/inet256"
)

type VolumeConfig struct {
	NodeID        blobcache.NodeID `json:"node"`
	VolumeID      blobcache.OID    `json:"volume"`
	DEK           blobcache.DEK    `json:"dek"`
	PrivateKeyHex string           `json:"private_key"`
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

func newMachines(fp FSParams) *machines {
	const (
		filedata = "filedata"
		inodekv  = "inodekv"
	)
	var dataSalt [32]byte
	gdat.DeriveKey(dataSalt[:], &fp.Salt, []byte(filedata))
	var inokvSalt [32]byte
	gdat.DeriveKey(inokvSalt[:], &fp.Salt, []byte(inodekv))
	return &machines{
		fdata: *gdat.NewMachine(gdat.Params{
			Salt: dataSalt,
		}),
		inodekv: gotkv.NewMachine(gotkv.Params{
			Salt:     inokvSalt,
			MaxSize:  int(fp.MaxBlobSize),
			MeanSize: 1 << 13,
		}),
	}
}

// System manages a set of WebFS Volumes reachable from a single root Volume
type System struct {
	bc  blobcache.Service
	pki inet256.PKI

	mu    sync.Mutex
	machs map[blobcache.FQOID]*machines
}

func NewSystem(svc blobcache.Service) *System {
	pki := inet256.PKI{
		Default: "mldsa87",
		Schemes: map[string]sign.Scheme{
			"mldsa87": mldsa87.Scheme(),
		},
	}
	return &System{
		pki: pki,
		bc:  svc,
	}
}

func (sys *System) Initialize(ctx context.Context, volh blobcache.Handle) (VolumeConfig, error) {
	var secret [32]byte
	// signing
	rand.Read(secret[:])
	_, privKey := mldsa87.NewKeyFromSeed(&secret)
	privKeyData, err := sys.pki.MarshalPrivateKey(nil, privKey)
	if err != nil {
		return VolumeConfig{}, err
	}

	// aead
	rand.Read(secret[:])
	dek := secret

	// setup volume
	if err := func() error {
		tx, err := bcsdk.BeginTx(ctx, sys.bc, volh, blobcache.TxParams{Modify: true})
		if err != nil {
			return err
		}
		var data []byte
		if err := tx.Load(ctx, &data); err != nil {
			return err
		}
		if len(data) != 0 {
			return fmt.Errorf("volume cell must be empty to be initialized")
		}
		var salt [32]byte
		rand.Read(salt[:])
		fsp := FSParams{
			Salt:        salt,
			MaxBlobSize: uint32(tx.MaxSize()),
		}
		root, err := sys.NewEmpty(ctx, tx, fsp)
		if err != nil {
			return err
		}
		// setup root
		tx2 := newTx(root, tx, tx, newMachines(fsp))
		_, seg := capnp.NewSingleSegmentMessage(nil)
		node, err := wfscnp.NewRootNode(seg)
		if err != nil {
			return err
		}
		node.Payload().NewDir()
		if err := tx2.setRoot(ctx, node); err != nil {
			return err
		}
		root, err = tx2.Flush(ctx)
		if err != nil {
			return err
		}
		if err := SaveRoot(ctx, tx, root); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}(); err != nil {
		return VolumeConfig{}, err
	}

	ep, err := sys.bc.Endpoint(ctx)
	if err != nil {
		return VolumeConfig{}, err
	}
	return VolumeConfig{
		NodeID:        ep.Node,
		VolumeID:      volh.OID,
		PrivateKeyHex: hex.EncodeToString(privKeyData),
		DEK:           dek,
	}, nil
}

func (sys *System) View(ctx context.Context, vcfg VolumeConfig, fn func(*Tx) error) error {
	volh, err := sys.bc.OpenFiat(ctx, vcfg.VolumeID, blobcache.Action_ALL)
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

func (sys *System) Modify(ctx context.Context, vcfg VolumeConfig, fn func(*Tx) error) error {
	volh, err := sys.bc.OpenFiat(ctx, vcfg.VolumeID, blobcache.Action_ALL)
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

// FSParams contains filesystem level parameters.
type FSParams struct {
	Salt        [32]byte
	MaxBlobSize uint32
}

func (sys *System) NewEmpty(ctx context.Context, txn *bcsdk.Tx, fp FSParams) (Root, error) {
	mach := newMachines(fp)
	inodeRoot, err := mach.inodekv.NewEmpty(ctx, txn)
	if err != nil {
		return Root{}, err
	}
	return Root{
		version:     0,
		maxBlobSize: fp.MaxBlobSize,
		salt:        fp.Salt,
		inodes:      inodeRoot,
	}, nil
}

func (sys *System) getMachs(fqoid blobcache.FQOID, fp FSParams) *machines {
	sys.mu.Lock()
	defer sys.mu.Unlock()
	if sys.machs == nil {
		sys.machs = make(map[blobcache.FQOID]*machines)
	}
	if machs, exists := sys.machs[fqoid]; exists {
		return machs
	}
	machs := newMachines(fp)
	sys.machs[fqoid] = machs
	return machs
}

func (sys *System) wrapTx(ctx context.Context, txn *bcsdk.Tx, fqoid blobcache.FQOID) (*Tx, error) {
	root, err := LoadRoot(ctx, txn)
	if err != nil {
		return nil, err
	}
	machs := sys.getMachs(fqoid, FSParams{MaxBlobSize: root.maxBlobSize, Salt: root.salt})
	return newTx(root, txn, txn, machs), nil
}

// Tx is a transaction on a webfs volume.
type Tx struct {
	root Root
	ros  bcsdk.RO
	rws  bcsdk.RW
	link Linker

	fdata      *gdat.Machine
	inodetx    *gotkv.Tx
	inodeCache map[INode]wfscnp.Node
}

type Linker interface {
	Link(ctx context.Context, target blobcache.Handle, mask blobcache.ActionSet) (*blobcache.LinkToken, error)
	Unlink(ctx context.Context, targets []blobcache.LinkTokenID) error
}

func newTx(root Root, s bcsdk.RW, link Linker, machs *machines) *Tx {
	return &Tx{
		root: root,
		ros:  s,
		rws:  s,
		link: link,

		fdata:   &machs.fdata,
		inodetx: machs.inodekv.NewTx(s, root.inodes),
	}
}

// Flush writes out the changes to the store and returns a new root.
func (tx *Tx) Flush(ctx context.Context) (Root, error) {
	inodekvroot, err := tx.inodetx.Flush(ctx)
	if err != nil {
		return Root{}, err
	}
	tx.root.inodes = inodekvroot
	return tx.root, nil
}

func (tx *Tx) getNode(ctx context.Context, ino INode) (wfscnp.Node, error) {
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

func (tx *Tx) putNode(ctx context.Context, ino INode, node wfscnp.Node) error {
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

func (tx *Tx) setRoot(ctx context.Context, node wfscnp.Node) error {
	return tx.putNode(ctx, rootINode(), node)
}
