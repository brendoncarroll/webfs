package webfs

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/fs"
	"math"
	"slices"

	capnp "capnproto.org/go/capnp/v3"
	"github.com/brendoncarroll/webfs/src/internal/wfscnp"
	"github.com/gotvc/got/src/gdat"
	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/exp/sbe"
	"go.brendoncarroll.net/exp/streams"
	"go.brendoncarroll.net/tai64"
	"go.inet256.org/inet256/src/inet256"
)

var sessionSigCtx = inet256.SigCtxString("webfs/session")

// ensureSession assumes the lock is held
func (tx *FSTx) ensureSession(ctx context.Context, privateKey inet256.PrivateKey, now tai64.TAI64N) (inet256.ID, error) {
	if tx.pki == nil {
		return inet256.ID{}, fmt.Errorf("tx has no pki")
	}
	if tx.gid != tx.prev.gid {
		return inet256.ID{}, fmt.Errorf("tx gid does not match state gid")
	}
	publicKey := inet256.PublicFromPrivate(privateKey)
	id := tx.pki.NewID(publicKey)
	_, err := tx.editSession(ctx, id, privateKey, func(session wfscnp.Session) error {
		if !session.HasCreateAt() {
			createdAt, err := session.NewCreateAt()
			if err != nil {
				return err
			}
			setCNPTime(createdAt, now)
		}
		touchedAt, err := session.NewTouchedAt()
		if err != nil {
			return err
		}
		setCNPTime(touchedAt, now)
		return nil
	})
	return id, err
}

// dropSession assumes the lock is held
func (tx *FSTx) dropSession(ctx context.Context, id inet256.ID) error {
	session, err := tx.getSession(ctx, id)
	if err != nil {
		return err
	}
	if session.LockCount() != 0 {
		if err := tx.deleteLocksForSession(ctx, id); err != nil {
			return err
		}
	}
	return tx.sessiontx.Delete(ctx, id[:])
}

func (tx *FSTx) SessionExists(ctx context.Context, id inet256.ID) (bool, error) {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	var val []byte
	return tx.sessiontx.Get(ctx, id[:], &val)
}

// GCSessions deletes sessions whose touchedAt + ttl has elapsed.
// A ttl of zero is treated as no expiry.
func (tx *FSTx) GCSessions(ctx context.Context) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if _, err := tx.sessiontx.Flush(ctx); err != nil {
		return err
	}
	now := tai64.Now()
	it := tx.sessiontx.IterateFlushed(ctx, gotkv.TotalSpan())
	buf := make([]gotkv.Entry, 32)
	for {
		n, err := it.Next(ctx, buf)
		if err != nil {
			if streams.IsEOS(err) {
				return nil
			}
			return err
		}
		for i := 0; i < n; i++ {
			if len(buf[i].Key) != len(inet256.ID{}) {
				return fmt.Errorf("invalid session key length: %d", len(buf[i].Key))
			}
			var sessionID inet256.ID
			copy(sessionID[:], buf[i].Key)
			sessionData, pubKey, err := openSigned(ctx, tx.pkcache, &sessionSigCtx, tx.ros, buf[i].Value)
			if err != nil {
				return err
			}
			if tx.pki.NewID(pubKey) != sessionID {
				return fmt.Errorf("session %v has wrong public key", sessionID)
			}
			session, err := parseSession(sessionData)
			if err != nil {
				return err
			}
			expired, err := sessionExpired(session, now)
			if err != nil {
				return err
			}
			if expired {
				if err := tx.sessiontx.Delete(ctx, slices.Clone(buf[i].Key)); err != nil {
					return err
				}
			}
		}
	}
}

// getSession reads and validates a session
// getSession assumes the lock is held
func (tx *FSTx) getSession(ctx context.Context, sessionID inet256.ID) (wfscnp.Session, error) {
	var value []byte
	exists, err := tx.sessiontx.Get(ctx, sessionID[:], &value)
	if err != nil {
		return wfscnp.Session{}, err
	}
	if !exists {
		return wfscnp.Session{}, fs.ErrNotExist
	}
	sessionData, pubKey, err := openSigned(ctx, tx.pkcache, &sessionSigCtx, tx.ros, value)
	if err != nil {
		return wfscnp.Session{}, err
	}
	actualID := tx.pki.NewID(pubKey)
	if actualID != sessionID {
		return wfscnp.Session{}, fmt.Errorf("session has wrong public key")
	}
	return parseSession(sessionData)
}

func (tx *FSTx) putSession(ctx context.Context, privateKey inet256.PrivateKey, x wfscnp.Session) error {
	sessionID := tx.pki.NewID(inet256.PublicFromPrivate(privateKey))
	sessionData, err := x.Message().Marshal()
	if err != nil {
		return err
	}
	val, err := sealSigned(ctx, tx.pkcache, &sessionSigCtx, tx.rws, privateKey, sessionData, nil)
	if err != nil {
		return err
	}
	if err := tx.sessiontx.Put(ctx, sessionID[:], val); err != nil {
		return err
	}
	return nil
}

// editSession assumes the lock is held
func (tx *FSTx) editSession(ctx context.Context, sessionID inet256.ID, privateKey inet256.PrivateKey, fn func(wfscnp.Session) error) (wfscnp.Session, error) {
	session, err := tx.getSession(ctx, sessionID)
	if err != nil && !fsErrNotExist(err) {
		return wfscnp.Session{}, err
	}
	missing := fsErrNotExist(err)
	_, seg := capnp.NewSingleSegmentMessage(nil)
	editable, err := wfscnp.NewRootSession(seg)
	if err != nil {
		return wfscnp.Session{}, err
	}
	if missing {
		session = editable
	} else {
		if err := copySession(editable, session); err != nil {
			return wfscnp.Session{}, err
		}
		session = editable
	}
	if err := fn(session); err != nil {
		return wfscnp.Session{}, err
	}
	sessionData, err := session.Message().Marshal()
	if err != nil {
		return wfscnp.Session{}, err
	}
	val, err := sealSigned(ctx, tx.pkcache, &sessionSigCtx, tx.rws, privateKey, sessionData, nil)
	if err != nil {
		return wfscnp.Session{}, err
	}
	if err := tx.sessiontx.Put(ctx, sessionID[:], val); err != nil {
		return wfscnp.Session{}, err
	}
	return session, nil
}

// incrementSessionLockCount
// assumes lock is held
func (tx *FSTx) incrementSessionLockCount(ctx context.Context, sessionID inet256.ID, delta int32) error {
	privateKey, err := tx.getSessionPrivateKey(ctx, sessionID)
	if err != nil {
		return err
	}
	_, err = tx.editSession(ctx, sessionID, privateKey, func(session wfscnp.Session) error {
		next := int64(session.LockCount()) + int64(delta)
		if next < 0 {
			return fmt.Errorf("session lock count underflow for %v", sessionID)
		}
		session.SetLockCount(uint32(next))
		return nil
	})
	return err
}

func parseSession(data []byte) (wfscnp.Session, error) {
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return wfscnp.Session{}, err
	}
	sess, err := wfscnp.ReadRootSession(msg)
	if err != nil {
		return wfscnp.Session{}, err
	}
	return sess, nil
}

// getSessionPublicKey retrieves the public key for a session.
// it assumes the read lock is held
func (tx *FSTx) getSessionPublicKey(ctx context.Context, id inet256.ID) (inet256.PublicKey, error) {
	var value []byte
	if ok, err := tx.sessiontx.Get(ctx, id[:], &value); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("session not found")
	}
	refData, _, err := sbe.ReadN(value, gdat.RefSize)
	if err != nil {
		return nil, err
	}
	ref, err := gdat.ParseRef(refData)
	if err != nil {
		return nil, err
	}
	return tx.pkcache.Get(ctx, tx.ros, ref)
}

func (tx *FSTx) getSessionPrivateKey(ctx context.Context, sessionID inet256.ID) (inet256.PrivateKey, error) {
	if tx.priv == nil {
		return nil, fmt.Errorf("tx has no private key")
	}
	if tx.pki.NewID(inet256.PublicFromPrivate(tx.priv)) != sessionID {
		return nil, fmt.Errorf("volume private key does not match session %v", sessionID)
	}
	return tx.priv, nil
}

func sessionSigMessage(gid [32]byte, sessionData []byte) []byte {
	msg := make([]byte, 0, len(gid)+len(sessionData))
	msg = append(msg, gid[:]...)
	msg = append(msg, sessionData...)
	return msg
}

func sessionExpired(session wfscnp.Session, now tai64.TAI64N) (bool, error) {
	ttl := session.Ttl()
	if ttl == 0 {
		return false, nil
	}
	base, err := sessionTimeBase(session)
	if err != nil {
		return false, err
	}
	if base.Seconds() > math.MaxUint64-uint64(ttl) {
		return false, nil
	}
	deadline := tai64.TAI64N{Seconds: base.Seconds() + uint64(ttl), Nanoseconds: base.Nanoseconds()}
	return !deadline.After(now), nil
}

func sessionTimeBase(session wfscnp.Session) (wfscnp.TAI64N, error) {
	if session.HasTouchedAt() {
		return session.TouchedAt()
	}
	return session.CreateAt()
}

func copySession(dst, src wfscnp.Session) error {
	createAt, err := src.CreateAt()
	if err != nil {
		return err
	}
	newCreateAt, err := dst.NewCreateAt()
	if err != nil {
		return err
	}
	copyCNPTime(newCreateAt, createAt)
	dst.SetTtl(src.Ttl())
	if src.HasTouchedAt() {
		touchedAt, err := src.TouchedAt()
		if err != nil {
			return err
		}
		newTouchedAt, err := dst.NewTouchedAt()
		if err != nil {
			return err
		}
		copyCNPTime(newTouchedAt, touchedAt)
	}
	dst.SetLockCount(src.LockCount())
	return nil
}

func setCNPTime(dst wfscnp.TAI64N, t tai64.TAI64N) {
	dst.SetSeconds(t.Seconds)
	dst.SetNanoseconds(t.Nanoseconds)
}

func copyCNPTime(dst wfscnp.TAI64N, src wfscnp.TAI64N) {
	dst.SetSeconds(src.Seconds())
	dst.SetNanoseconds(src.Nanoseconds())
}

func setCNPRef(dst wfscnp.Ref, ref gdat.Ref) error {
	cid, err := dst.NewCid()
	if err != nil {
		return err
	}
	setCNP256(cid, ref.CID[:])
	dek, err := dst.NewDek()
	if err != nil {
		return err
	}
	setCNP256(dek, ref.DEK[:])
	return nil
}

func copyCNPRef(dst wfscnp.Ref, src wfscnp.Ref) error {
	cid, err := src.Cid()
	if err != nil {
		return err
	}
	newCID, err := dst.NewCid()
	if err != nil {
		return err
	}
	copyCNP256(newCID, cid)
	dek, err := src.Dek()
	if err != nil {
		return err
	}
	newDEK, err := dst.NewDek()
	if err != nil {
		return err
	}
	copyCNP256(newDEK, dek)
	return nil
}

func cnpRefToGdatRef(src wfscnp.Ref) (gdat.Ref, error) {
	cid, err := src.Cid()
	if err != nil {
		return gdat.Ref{}, err
	}
	dek, err := src.Dek()
	if err != nil {
		return gdat.Ref{}, err
	}
	return gdat.Ref{CID: cnp256ToArray(cid), DEK: cnp256ToArray(dek)}, nil
}

func setCNP256(dst wfscnp.UInt256, data []byte) {
	dst.SetA(binary.LittleEndian.Uint64(data[:8]))
	dst.SetB(binary.LittleEndian.Uint64(data[8:16]))
	dst.SetC(binary.LittleEndian.Uint64(data[16:24]))
	dst.SetD(binary.LittleEndian.Uint64(data[24:32]))
}

func copyCNP256(dst wfscnp.UInt256, src wfscnp.UInt256) {
	dst.SetA(src.A())
	dst.SetB(src.B())
	dst.SetC(src.C())
	dst.SetD(src.D())
}

func cnp256ToArray(src wfscnp.UInt256) (ret [32]byte) {
	binary.LittleEndian.PutUint64(ret[:8], src.A())
	binary.LittleEndian.PutUint64(ret[8:16], src.B())
	binary.LittleEndian.PutUint64(ret[16:24], src.C())
	binary.LittleEndian.PutUint64(ret[24:32], src.D())
	return ret
}
