package webfs

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/fs"
	"maps"
	"sync"

	"blobcache.io/blobcache/src/bcsdk"
	"blobcache.io/blobcache/src/blobcache"
	capnp "capnproto.org/go/capnp/v3"
	"github.com/brendoncarroll/webfs/src/internal/wfscnp"
	"github.com/cloudflare/circl/sign"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/gotvc/got/src/gdat"
	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/tai64"
	"go.inet256.org/inet256/src/inet256"
)

type GID = blobcache.CID

const MLDSA87 = "mldsa87"

type VolumeConfig struct {
	NodeID         blobcache.NodeID   `json:"node"`
	VolumeID       blobcache.OID      `json:"volume"`
	HashAlgo       blobcache.HashAlgo `json:"hash_algo"`
	GID            GID                `json:"gid"`
	DEK            blobcache.DEK      `json:"dek"`
	PrivateKeySeed blobcache.DEK      `json:"private"`
	// SignAlgo is the signature algorithm to act as.
	SignAlgo string `json:"sign_algo"`
}

func (vc VolumeConfig) DeriveSiging() (sign.PublicKey, sign.PrivateKey) {
	return mldsa87.NewKeyFromSeed((*[32]byte)(&vc.PrivateKeySeed))
}

func (vc VolumeConfig) FQOID() blobcache.FQOID {
	return blobcache.FQOID{Node: vc.NodeID, OID: vc.VolumeID}
}

func DefaultPKI() inet256.PKI {
	return inet256.PKI{
		Default: MLDSA87,
		Schemes: map[string]sign.Scheme{
			MLDSA87: mldsa87.Scheme(),
		},
	}
}

type machines struct {
	// fdata manages posting and getting file data
	fdata gdat.Machine
	// inodekv manages interactions with the inode table.
	inodekv gotkv.Machine
	// xattrkv manages interactions with the xattrs table.
	xattrkv gotkv.Machine
	// sessionkv manages interactions with the sessions table.
	sessionkv gotkv.Machine
	// lockkv manages interactions with the locks table.
	lockkv gotkv.Machine
}

func newMachines(fp FSParams) *machines {
	const (
		filedata  = "filedata"
		inodekv   = "inodekv"
		xattrkv   = "xattrkv"
		sessionkv = "sessionkv"
		lockkv    = "lockkv"
	)
	var dataSalt [32]byte
	gdat.DeriveKey(dataSalt[:], &fp.Salt, []byte(filedata))
	var inokvSalt [32]byte
	gdat.DeriveKey(inokvSalt[:], &fp.Salt, []byte(inodekv))
	var xattrkvSalt [32]byte
	gdat.DeriveKey(xattrkvSalt[:], &fp.Salt, []byte(xattrkv))
	var sessionkvSalt [32]byte
	gdat.DeriveKey(sessionkvSalt[:], &fp.Salt, []byte(sessionkv))
	var lockkvSalt [32]byte
	gdat.DeriveKey(lockkvSalt[:], &fp.Salt, []byte(lockkv))
	return &machines{
		fdata: *gdat.NewMachine(gdat.Params{
			Salt:          dataSalt,
			KeyedHashFunc: fp.HashAlgo.KeyedHash,
		}),
		inodekv: gotkv.NewMachine(gotkv.Params{
			Salt:          inokvSalt,
			MaxSize:       int(fp.MaxBlobSize),
			MeanSize:      1 << 13,
			KeyedHashFunc: fp.HashAlgo.KeyedHash,
		}),
		xattrkv: gotkv.NewMachine(gotkv.Params{
			Salt:          xattrkvSalt,
			MaxSize:       int(fp.MaxBlobSize),
			MeanSize:      1 << 13,
			KeyedHashFunc: fp.HashAlgo.KeyedHash,
		}),
		sessionkv: gotkv.NewMachine(gotkv.Params{
			Salt:          sessionkvSalt,
			MaxSize:       int(fp.MaxBlobSize),
			MeanSize:      1 << 13,
			KeyedHashFunc: fp.HashAlgo.KeyedHash,
		}),
		lockkv: gotkv.NewMachine(gotkv.Params{
			Salt:          lockkvSalt,
			MaxSize:       int(fp.MaxBlobSize),
			MeanSize:      1 << 13,
			KeyedHashFunc: fp.HashAlgo.KeyedHash,
		}),
	}
}

// System manages a set of WebFS Volumes
// Caches and configuration are managed for each Volume.
type System struct {
	bc  blobcache.Service
	pki inet256.PKI

	mu    sync.Mutex
	machs map[blobcache.FQOID]*machines
}

func NewSystem(svc blobcache.Service, pki inet256.PKI) *System {
	return &System{pki: pki, bc: svc}
}

func (sys *System) PKI() inet256.PKI {
	return inet256.PKI{
		Default: sys.pki.Default,
		Schemes: maps.Clone(sys.pki.Schemes),
	}
}

func (sys *System) GenerateConfig(fqoid blobcache.FQOID) VolumeConfig {
	var asymSeed [32]byte
	rand.Read(asymSeed[:])
	// signing
	mldsa87.NewKeyFromSeed(&asymSeed)

	// aead
	var aeadSecret [32]byte
	rand.Read(aeadSecret[:])
	// gid
	var gid [32]byte
	rand.Read(gid[:])

	const hashAlgo = blobcache.HashAlgo_BLAKE3_256
	return VolumeConfig{
		VolumeID:       fqoid.OID,
		NodeID:         fqoid.Node,
		HashAlgo:       hashAlgo,
		GID:            gid,
		DEK:            aeadSecret,
		PrivateKeySeed: asymSeed,
		SignAlgo:       MLDSA87,
	}
}

// Initialize initializes a new webfs filesystem in a volume using config.
// The volume must have an empty cell.
func (sys *System) Initialize(ctx context.Context, volh blobcache.Handle, cfg VolumeConfig) error {
	var salt [32]byte
	rand.Read(salt[:])
	// setup volume
	if err := func() error {
		tx, err := bcsdk.BeginTx(ctx, sys.bc, volh, blobcache.TxParams{Modify: true})
		if err != nil {
			return err
		}
		defer tx.Abort(ctx)
		var data []byte
		if err := tx.Load(ctx, &data); err != nil {
			return err
		}
		if len(data) != 0 {
			return fmt.Errorf("volume cell must be empty to be initialized")
		}
		fsp := FSParams{
			HashAlgo:    cfg.HashAlgo,
			GID:         cfg.GID,
			Salt:        salt,
			MaxBlobSize: uint32(tx.MaxSize()),
		}
		root, err := sys.NewEmpty(ctx, tx, fsp)
		if err != nil {
			return err
		}
		// setup root
		tx2 := newTx(root, tx, tx, newMachines(fsp), &sys.pki, nil)
		_, seg := capnp.NewSingleSegmentMessage(nil)
		node, err := wfscnp.NewRootNode(seg)
		if err != nil {
			return err
		}
		node.Payload().NewDir()
		setNodeMode(node, fs.ModeDir|0o755)
		if err := tx2.setRoot(ctx, node); err != nil {
			return err
		}
		root, err = tx2.Flush(ctx)
		if err != nil {
			return err
		}
		if err := SaveState(ctx, tx, root); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}(); err != nil {
		return err
	}
	return nil
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
	privKey := deriveVolumePrivateKey(&sys.pki, vcfg)
	txn2, err := sys.wrapTx(ctx, txn, fqoid, vcfg.HashAlgo, privKey)
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
	privKey := deriveVolumePrivateKey(&sys.pki, vcfg)
	txn2, err := sys.wrapTx(ctx, txn, fqoid, vcfg.HashAlgo, privKey)
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
	HashAlgo    blobcache.HashAlgo
	GID         GID
	Salt        [32]byte
	MaxBlobSize uint32
}

func (sys *System) NewEmpty(ctx context.Context, txn *bcsdk.Tx, fp FSParams) (FSState, error) {
	mach := newMachines(fp)
	inodeRoot, err := mach.inodekv.NewEmpty(ctx, txn)
	if err != nil {
		return FSState{}, err
	}
	xattrRoot, err := mach.xattrkv.NewEmpty(ctx, txn)
	if err != nil {
		return FSState{}, err
	}
	sessionRoot, err := mach.sessionkv.NewEmpty(ctx, txn)
	if err != nil {
		return FSState{}, err
	}
	lockRoot, err := mach.lockkv.NewEmpty(ctx, txn)
	if err != nil {
		return FSState{}, err
	}
	return FSState{
		version:     0,
		maxBlobSize: fp.MaxBlobSize,
		gid:         fp.GID,
		salt:        fp.Salt,
		inodes:      inodeRoot,
		xattrs:      xattrRoot,
		sessions:    sessionRoot,
		locks:       lockRoot,
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

func (sys *System) wrapTx(ctx context.Context, txn *bcsdk.Tx, fqoid blobcache.FQOID, hashAlgo blobcache.HashAlgo, privKey inet256.PrivateKey) (*Tx, error) {
	root, err := LoadState(ctx, txn)
	if err != nil {
		return nil, err
	}
	machs := sys.getMachs(fqoid, FSParams{HashAlgo: hashAlgo, GID: root.gid, MaxBlobSize: root.maxBlobSize, Salt: root.salt})
	return newTx(root, txn, txn, machs, &sys.pki, privKey), nil
}

type Linker interface {
	Link(ctx context.Context, target blobcache.Handle, mask blobcache.ActionSet) (*blobcache.LinkToken, error)
	Unlink(ctx context.Context, targets []blobcache.LinkTokenID) error
}

// Tx is a transaction on a webfs volume.
type Tx struct {
	// prev is the previous existing state, without any pending changes
	prev FSState
	ros  bcsdk.RO
	rws  bcsdk.RW
	link Linker
	gid  GID
	pki  *inet256.PKI
	priv inet256.PrivateKey

	fdata      *gdat.Machine
	inodetx    *gotkv.Tx
	xattrtx    *gotkv.Tx
	sessiontx  *gotkv.Tx
	locktx     *gotkv.Tx
	inodeCache map[INode]wfscnp.Node
}

type INodeStats struct {
	RefCount uint32
}

func newTx(prev FSState, s bcsdk.RW, link Linker, machs *machines, pki *inet256.PKI, priv inet256.PrivateKey) *Tx {
	return &Tx{
		prev: prev,
		ros:  s,
		rws:  s,
		link: link,
		gid:  prev.gid,
		pki:  pki,
		priv: priv,

		fdata:     &machs.fdata,
		inodetx:   machs.inodekv.NewTx(s, prev.inodes),
		xattrtx:   machs.xattrkv.NewTx(s, prev.xattrs),
		sessiontx: machs.sessionkv.NewTx(s, prev.sessions),
		locktx:    machs.lockkv.NewTx(s, prev.locks),
	}
}

func deriveVolumePrivateKey(pki *inet256.PKI, vcfg VolumeConfig) inet256.PrivateKey {
	_, priv := mldsa87.NewKeyFromSeed((*[32]byte)(&vcfg.PrivateKeySeed))
	return priv
}

// Flush writes out the changes to the store and returns a new root.
func (tx *Tx) Flush(ctx context.Context) (FSState, error) {
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
	return tx.putNode(ctx, INode{}, node)
}

func (tx *Tx) StatINode(ctx context.Context, ino INode) (INodeStats, error) {
	node, err := tx.getNode(ctx, ino)
	if err != nil {
		return INodeStats{}, err
	}
	return INodeStats{RefCount: node.RefCount()}, nil
}

func (tx *Tx) GetModifiedAt(ctx context.Context, ino INode) (tai64.TAI64N, error) {
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

func (tx *Tx) SetModifiedAt(ctx context.Context, ino INode, t tai64.TAI64N) error {
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
