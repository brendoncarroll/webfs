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
	"go.inet256.org/inet256/src/inet256"
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
