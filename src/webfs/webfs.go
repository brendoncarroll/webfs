package webfs

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/fs"
	"maps"
	"slices"
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
	"golang.org/x/crypto/chacha20poly1305"
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
	// Authors is a list of keys which are allowed to author new filesystems
	Authors []inet256.ID `json:"authors"`
}

func (vc VolumeConfig) DeriveSiging() (sign.PublicKey, sign.PrivateKey) {
	seed := vc.HashAlgo.KeyedHash((*blobcache.CID)(&vc.PrivateKeySeed), []byte(MLDSA87))
	return mldsa87.NewKeyFromSeed((*[32]byte)(&seed))
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

	mu       sync.Mutex
	machs    map[blobcache.FQOID]*machines
	pkcaches map[blobcache.FQOID]*gdatcache.Cache[inet256.PublicKey]
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
	// private
	var private [32]byte
	rand.Read(private[:])
	// aead
	var aeadSecret [32]byte
	rand.Read(aeadSecret[:])
	// gid
	var gid [32]byte
	rand.Read(gid[:])

	const hashAlgo = blobcache.HashAlgo_BLAKE3_256
	vcfg := VolumeConfig{
		VolumeID:       fqoid.OID,
		NodeID:         fqoid.Node,
		HashAlgo:       hashAlgo,
		GID:            gid,
		DEK:            aeadSecret,
		PrivateKeySeed: private,
		SignAlgo:       MLDSA87,
	}
	pub, _ := vcfg.DeriveSiging()
	id := sys.pki.NewID(pub)
	vcfg.Authors = []inet256.ID{id}
	return vcfg
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
		tx2 := sys.newFSTx(cfg, root, tx, tx)
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
		out, err := sys.sealFS(ctx, cfg, tx, root)
		if err != nil {
			return err
		}
		if err := tx.Save(ctx, out); err != nil {
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
	return sys.beginTx(ctx, vcfg, txn, false)
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
	return sys.beginTx(ctx, vcfg, txn, true)
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
	extRoot, err := mach.exts.NewEmpty(ctx, txn)
	if err != nil {
		return FSState{}, err
	}
	dirEntRoot, err := mach.dirEnts.NewEmpty(ctx, txn)
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
		exts:        extRoot,
		dirEnts:     dirEntRoot,
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

func (sys *System) getPKCache(fqoid blobcache.FQOID, hashAlgo blobcache.HashAlgo) *gdatcache.Cache[inet256.PublicKey] {
	sys.mu.Lock()
	defer sys.mu.Unlock()
	pkcache := sys.pkcaches[fqoid]
	if pkcache == nil {
		dmach := newDataMach([32]byte{}, hashAlgo)
		pkcache = newPublicKeyCache(dmach, &sys.pki, 32)
		if sys.pkcaches == nil {
			sys.pkcaches = map[blobcache.FQOID]*PublicKeyCache{}
		}
		sys.pkcaches[fqoid] = pkcache
	}
	return pkcache
}

type INodeStats struct {
	RefCount uint32
}

func derivePrivateKey(pki *inet256.PKI, vcfg VolumeConfig) inet256.PrivateKey {
	_, priv := vcfg.DeriveSiging()
	return priv
}

type machines struct {
	// fdata manages posting and getting file data
	fdata gdat.Machine
	// inodekv manages interactions with the inode table.
	inodekv gotkv.Machine
	// exts manages interactions with the extents table.
	exts gotkv.Machine
	// dirEnts manages interactions with the dir entries table.
	dirEnts gotkv.Machine
	// xattrkv manages interactions with the xattrs table.
	xattrkv gotkv.Machine
	// sessionkv manages interactions with the sessions table.
	sessionkv gotkv.Machine
	// lockkv manages interactions with the locks table.
	lockkv gotkv.Machine
}

func newDataMach(dataSalt [32]byte, hashAlgo blobcache.HashAlgo) *gdat.Machine {
	return gdat.NewMachine(gdat.Params{
		Salt:          dataSalt,
		KeyedHashFunc: hashAlgo.KeyedHash,
	})
}

func newMachines(fp FSParams) *machines {
	const (
		filedata  = "filedata"
		inodekv   = "inodekv"
		exts      = "exts"
		dirEnts   = "dirEnts"
		xattrkv   = "xattrkv"
		sessionkv = "sessionkv"
		lockkv    = "lockkv"
	)
	var dataSalt [32]byte
	gdat.DeriveKey(dataSalt[:], &fp.Salt, []byte(filedata))
	var inokvSalt [32]byte
	gdat.DeriveKey(inokvSalt[:], &fp.Salt, []byte(inodekv))
	var extsSalt [32]byte
	gdat.DeriveKey(extsSalt[:], &fp.Salt, []byte(exts))
	var dirEntsSalt [32]byte
	gdat.DeriveKey(dirEntsSalt[:], &fp.Salt, []byte(dirEnts))
	var xattrkvSalt [32]byte
	gdat.DeriveKey(xattrkvSalt[:], &fp.Salt, []byte(xattrkv))
	var sessionkvSalt [32]byte
	gdat.DeriveKey(sessionkvSalt[:], &fp.Salt, []byte(sessionkv))
	var lockkvSalt [32]byte
	gdat.DeriveKey(lockkvSalt[:], &fp.Salt, []byte(lockkv))
	return &machines{
		fdata: *newDataMach(dataSalt, fp.HashAlgo),
		inodekv: gotkv.NewMachine(gotkv.Params{
			Salt:          inokvSalt,
			MaxSize:       int(fp.MaxBlobSize),
			MeanSize:      1 << 13,
			KeyedHashFunc: fp.HashAlgo.KeyedHash,
		}),
		exts: gotkv.NewMachine(gotkv.Params{
			Salt:          extsSalt,
			MaxSize:       int(fp.MaxBlobSize),
			MeanSize:      1 << 13,
			KeyedHashFunc: fp.HashAlgo.KeyedHash,
		}),
		dirEnts: gotkv.NewMachine(gotkv.Params{
			Salt:          dirEntsSalt,
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
	sem     semaphore.Weighted
	isWrite bool

	FSTx
}

func (sys *System) beginTx(ctx context.Context, vcfg VolumeConfig, bctx *bcsdk.Tx, isWrite bool) (*Tx, error) {
	var ctext []byte
	if err := bctx.Load(ctx, &ctext); err != nil {
		return nil, err
	}
	fqoid := blobcache.FQOID{OID: vcfg.VolumeID, Node: vcfg.NodeID}
	pkcache := sys.getPKCache(fqoid, vcfg.HashAlgo)
	fsstate, authorPub, err := openFS(ctx, pkcache, vcfg, bctx, ctext)
	if err != nil {
		return nil, err
	}
	authorID := sys.pki.NewID(authorPub)
	if !slices.Contains(vcfg.Authors, authorID) {
		return nil, &ErrNotAllowed{Actor: authorID, Op: "author_fs"}
	}
	fstx := sys.newFSTx(vcfg, fsstate, bctx, bctx)
	return &Tx{
		bctx:    bctx,
		FSTx:    *fstx,
		isWrite: isWrite,
		sem:     *semaphore.NewWeighted(1),
	}, nil
}

func (tx *Tx) save(ctx context.Context) error {
	fsstate, err := tx.FSTx.Flush(ctx)
	if err != nil {
		return err
	}
	ctext, err := tx.sys.sealFS(ctx, tx.vcfg, tx.bctx, fsstate)
	if err != nil {
		return err
	}
	return tx.bctx.Save(ctx, ctext)
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

func (tx *Tx) IsWrite() bool {
	return tx.isWrite
}

var fsSigCtx = inet256.SigCtxString("webfs/fs")

// sealFS takes an FS and signs and encrypts it.
func (sys *System) sealFS(ctx context.Context, vcfg VolumeConfig, s bcsdk.WO, x FSState) ([]byte, error) {
	_, priv := vcfg.DeriveSiging()
	pkcache := sys.getPKCache(vcfg.FQOID(), vcfg.HashAlgo)
	ptext, err := sealSigned(ctx, pkcache, &fsSigCtx, s, priv, x.Marshal(nil), nil)
	if err != nil {
		return nil, err
	}
	var nonce [chacha20poly1305.NonceSizeX]byte
	aead, err := chacha20poly1305.NewX(vcfg.DEK[:])
	if err != nil {
		return nil, err
	}
	ctext := aead.Seal(nonce[:], nonce[:], ptext, nil)
	return ctext, nil
}

func openFS(ctx context.Context, pkcache *PublicKeyCache, vcfg VolumeConfig, s bcsdk.RO, in []byte) (FSState, inet256.PublicKey, error) {
	aead, err := chacha20poly1305.NewX(vcfg.DEK[:])
	if err != nil {
		return FSState{}, nil, err
	}
	// get nonce from front
	if len(in) < chacha20poly1305.NonceSizeX {
		return FSState{}, nil, fmt.Errorf("too short to contain nonce")
	}
	var nonce [chacha20poly1305.NonceSizeX]byte
	copy(nonce[:], in[:])
	ctext := in[chacha20poly1305.NonceSizeX:]

	var ptext []byte
	ptext, err = aead.Open(ptext, nonce[:], ctext, nil)
	if err != nil {
		return FSState{}, nil, err
	}
	fsdata, authorPub, err := openSigned(ctx, pkcache, &fsSigCtx, s, ptext)
	if err != nil {
		return FSState{}, nil, err
	}
	var ret FSState
	if err := ret.Unmarshal(fsdata); err != nil {
		return FSState{}, nil, err
	}
	return ret, authorPub, nil
}

type PublicKeyCache = gdatcache.Cache[inet256.PublicKey]

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
