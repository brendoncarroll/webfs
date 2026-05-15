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
	"github.com/brendoncarroll/webfs/src/internal/gdatcache"
	"github.com/brendoncarroll/webfs/src/internal/wfscnp"
	"github.com/cloudflare/circl/sign"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/gotvc/got/src/gdat"
	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/exp/sbe"
	"go.inet256.org/inet256/src/inet256"
	"golang.org/x/sync/semaphore"
)

// GID is a globally unique ID that uniquely identifies the filesystem.
type GID = blobcache.CID

const MLDSA87 = "mldsa87"

type VolumeConfig struct {
	NodeID         blobcache.NodeID   `json:"node"`
	VolumeID       blobcache.OID      `json:"volume"`
	HashAlgo       blobcache.HashAlgo `json:"hash_algo"`
	GID            GID                `json:"gid"`
	DEK            blobcache.DEK      `json:"dek"`
	PrivateKeySeed blobcache.DEK      `json:"private"`
	// SignAlgo is the signature algorithm to use for signing.
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
		tx2 := newFSTx(root, tx, tx, newMachines(fsp), &sys.pki, nil)
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

func (sys *System) View(ctx context.Context, vcfg VolumeConfig) (*Tx, error) {
	volh, err := sys.bc.OpenFiat(ctx, vcfg.VolumeID, blobcache.Action_ALL)
	if err != nil {
		return nil, err
	}
	txn, err := bcsdk.BeginTx(ctx, sys.bc, *volh, blobcache.TxParams{})
	if err != nil {
		return nil, err
	}
	return sys.beginTx(ctx, vcfg, txn)
}

func (sys *System) Modify(ctx context.Context, vcfg VolumeConfig) (*Tx, error) {
	volh, err := sys.bc.OpenFiat(ctx, vcfg.VolumeID, blobcache.Action_ALL)
	if err != nil {
		return nil, err
	}
	txn, err := bcsdk.BeginTx(ctx, sys.bc, *volh, blobcache.TxParams{Modify: true})
	if err != nil {
		return nil, err
	}
	return sys.beginTx(ctx, vcfg, txn)
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

type INodeStats struct {
	RefCount uint32
}

func deriveVolumePrivateKey(pki *inet256.PKI, vcfg VolumeConfig) inet256.PrivateKey {
	_, priv := mldsa87.NewKeyFromSeed((*[32]byte)(&vcfg.PrivateKeySeed))
	return priv
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

// Tx is a WebFS transaction.
type Tx struct {
	bctx *bcsdk.Tx
	// sem serializes calls to Abort and Commit
	sem semaphore.Weighted

	FSTx
}

func (sys *System) beginTx(ctx context.Context, vcfg VolumeConfig, bctx *bcsdk.Tx) (*Tx, error) {
	root, err := LoadState(ctx, bctx)
	if err != nil {
		return nil, err
	}
	fqoid := blobcache.FQOID{OID: vcfg.VolumeID, Node: vcfg.NodeID}
	fsp := FSParams{
		HashAlgo:    vcfg.HashAlgo,
		GID:         root.gid,
		MaxBlobSize: root.maxBlobSize,
		Salt:        root.salt,
	}
	machs := sys.getMachs(fqoid, fsp)
	privKey := deriveVolumePrivateKey(&sys.pki, vcfg)
	fstx := newFSTx(root, bctx, bctx, machs, &sys.pki, privKey)
	return &Tx{
		bctx: bctx,
		FSTx: *fstx,
		sem:  *semaphore.NewWeighted(1),
	}, nil
}

func (tx *Tx) save(ctx context.Context) error {
	fsstate, err := tx.FSTx.Flush(ctx)
	if err != nil {
		return err
	}
	return SaveState(ctx, tx.bctx, fsstate)
}

func (tx *Tx) Abort(ctx context.Context) error {
	if err := tx.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	defer tx.sem.Release(1)
	return tx.bctx.Abort(ctx)
}

func (tx *Tx) Commit(ctx context.Context) error {
	if err := tx.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	defer tx.sem.Release(1)
	if err := tx.save(ctx); err != nil {
		return err
	}
	return tx.bctx.Commit(ctx)
}

func newPublicKeyCache(mach *gdat.Machine, pki *inet256.PKI, size int) *gdatcache.Cache[inet256.PublicKey] {
	marshal := func(x inet256.PublicKey, out []byte) []byte {
		data, err := pki.MarshalPublicKey(out, x)
		if err != nil {
			panic(err)
		}
		return data
	}
	parse := func(data []byte) (inet256.PublicKey, error) {
		return pki.ParsePublicKey(data)
	}
	return gdatcache.New[inet256.PublicKey](mach, marshal, parse, size)
}

// openSigned verifies and returns a value from within a signed envelope
func openSigned(ctx context.Context, c *gdatcache.Cache[inet256.PublicKey], sigctx *inet256.SigCtx, s bcsdk.RO, data []byte) ([]byte, inet256.PublicKey, error) {
	refData, data, err := sbe.ReadN(data, gdat.RefSize)
	if err != nil {
		return nil, nil, err
	}
	ref, err := gdat.ParseRef(refData)
	if err != nil {
		return nil, nil, err
	}
	pubKey, err := c.Get(ctx, s, ref)
	if err != nil {
		return nil, nil, err
	}
	sigSize := pubKey.Scheme().SignatureSize()
	if len(data) < sigSize {
		return nil, nil, fmt.Errorf("too short to contain signature")
	}
	msg, sig := data[:len(data)-sigSize], data[len(data)-sigSize:]
	if !inet256.Verify(sigctx, pubKey, msg, sig) {
		return nil, nil, fmt.Errorf("invalid signature")
	}
	return msg, pubKey, nil
}

// sealSigned creates a new signed envelope and appends it to out.
func sealSigned(ctx context.Context, c *gdatcache.Cache[inet256.PublicKey], sigctx *inet256.SigCtx, s bcsdk.WO, privateKey inet256.PrivateKey, msg []byte, out []byte) ([]byte, error) {
	pubKey := inet256.PublicFromPrivate(privateKey)
	ref, err := c.Post(ctx, s, pubKey)
	if err != nil {
		return nil, err
	}
	out = append(out, ref.Marshal()...)
	out = append(out, msg...)
	out = inet256.Sign(sigctx, privateKey, msg, out)
	return out, nil
}
