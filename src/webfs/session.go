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

func (tx *Tx) ensureSession(ctx context.Context, privateKey inet256.PrivateKey, now tai64.TAI64N) (inet256.ID, error) {
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
			publicKeyData, err := tx.pki.MarshalPublicKey(nil, publicKey)
			if err != nil {
				return err
			}
			publicKeyRef, err := tx.fdata.Post(ctx, tx.rws, publicKeyData)
			if err != nil {
				return err
			}
			ref, err := session.NewPublicKeyRef()
			if err != nil {
				return err
			}
			if err := setCNPRef(ref, publicKeyRef); err != nil {
				return err
			}
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

func (tx *Tx) dropSession(ctx context.Context, id inet256.ID) error {
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

// GCSessions deletes sessions whose touchedAt + ttl has elapsed.
// A ttl of zero is treated as no expiry.
func (tx *Tx) GCSessions(ctx context.Context) error {
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
			session, err := tx.unmarshalSessionValue(buf[i].Key, buf[i].Value)
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

func (tx *Tx) getSession(ctx context.Context, sessionID inet256.ID) (wfscnp.Session, error) {
	var value []byte
	exists, err := tx.sessiontx.Get(ctx, sessionID[:], &value)
	if err != nil {
		return wfscnp.Session{}, err
	}
	if !exists {
		return wfscnp.Session{}, fs.ErrNotExist
	}
	return tx.unmarshalSessionValue(sessionID[:], value)
}

func (tx *Tx) editSession(ctx context.Context, sessionID inet256.ID, privateKey inet256.PrivateKey, fn func(wfscnp.Session) error) (wfscnp.Session, error) {
	var prev wfscnp.Session
	if session, err := tx.getSession(ctx, sessionID); err == nil {
		prev = session
	} else if !fsErrNotExist(err) {
		return wfscnp.Session{}, err
	}
	_, seg := capnp.NewSingleSegmentMessage(nil)
	session, err := wfscnp.NewRootSession(seg)
	if err != nil {
		return wfscnp.Session{}, err
	}
	if prev.IsValid() {
		if err := copySession(session, prev); err != nil {
			return wfscnp.Session{}, err
		}
	}
	if err := fn(session); err != nil {
		return wfscnp.Session{}, err
	}
	sessionData, err := session.Message().Marshal()
	if err != nil {
		return wfscnp.Session{}, err
	}
	sig := tx.pki.Sign(&sessionSigCtx, privateKey, sessionSigMessage(tx.gid, sessionData), nil)
	if err := tx.sessiontx.Put(ctx, sessionID[:], makeSessionValue(nil, sessionData, sig)); err != nil {
		return wfscnp.Session{}, err
	}
	return session, nil
}

func (tx *Tx) editSessionByID(ctx context.Context, sessionID inet256.ID, fn func(wfscnp.Session) error) (wfscnp.Session, error) {
	privateKey, err := tx.privateKeyForSession(ctx, sessionID)
	if err != nil {
		return wfscnp.Session{}, err
	}
	return tx.editSession(ctx, sessionID, privateKey, fn)
}

func (tx *Tx) unmarshalSessionValue(key []byte, value []byte) (wfscnp.Session, error) {
	sessionData, sig, err := parseSessionValue(value)
	if err != nil {
		return wfscnp.Session{}, err
	}
	id := inet256.IDFromBytes(key)
	publicKey, err := tx.sessionPublicKey(id, sessionData)
	if err != nil {
		return wfscnp.Session{}, err
	}
	if !tx.pki.Verify(&sessionSigCtx, publicKey, sessionSigMessage(tx.gid, sessionData), sig) {
		return wfscnp.Session{}, fmt.Errorf("invalid session signature for %v", id)
	}
	msg, err := capnp.Unmarshal(sessionData)
	if err != nil {
		return wfscnp.Session{}, err
	}
	return wfscnp.ReadRootSession(msg)
}

func (tx *Tx) sessionPublicKey(id inet256.ID, sessionData []byte) (inet256.PublicKey, error) {
	msg, err := capnp.Unmarshal(sessionData)
	if err != nil {
		return nil, err
	}
	session, err := wfscnp.ReadRootSession(msg)
	if err != nil {
		return nil, err
	}
	refCNP, err := session.PublicKeyRef()
	if err != nil {
		return nil, err
	}
	ref, err := cnpRefToGdatRef(refCNP)
	if err != nil {
		return nil, err
	}
	pubKeyData := make([]byte, tx.ros.MaxSize())
	n, err := tx.fdata.Read(context.Background(), tx.ros, ref, pubKeyData)
	if err != nil {
		return nil, err
	}
	pubKey, err := tx.pki.ParsePublicKey(pubKeyData[:n])
	if err != nil {
		return nil, err
	}
	if tx.pki.NewID(pubKey) != id {
		return nil, fmt.Errorf("session public key does not match id %v", id)
	}
	return pubKey, nil
}

func (tx *Tx) privateKeyForSession(ctx context.Context, sessionID inet256.ID) (inet256.PrivateKey, error) {
	if tx.priv == nil {
		return nil, fmt.Errorf("tx has no private key")
	}
	if tx.pki.NewID(inet256.PublicFromPrivate(tx.priv)) != sessionID {
		return nil, fmt.Errorf("volume private key does not match session %v", sessionID)
	}
	return tx.priv, nil
}

func (tx *Tx) publicKeyFromSession(sessionID inet256.ID, session wfscnp.Session) (inet256.PublicKey, error) {
	refCNP, err := session.PublicKeyRef()
	if err != nil {
		return nil, err
	}
	ref, err := cnpRefToGdatRef(refCNP)
	if err != nil {
		return nil, err
	}
	pubKeyData := make([]byte, tx.ros.MaxSize())
	n, err := tx.fdata.Read(context.Background(), tx.ros, ref, pubKeyData)
	if err != nil {
		return nil, err
	}
	pubKey, err := tx.pki.ParsePublicKey(pubKeyData[:n])
	if err != nil {
		return nil, err
	}
	if tx.pki.NewID(pubKey) != sessionID {
		return nil, fmt.Errorf("session public key does not match id %v", sessionID)
	}
	return pubKey, nil
}

func makeSessionValue(out []byte, sessionData []byte, sig []byte) []byte {
	out = sbe.AppendLP32(out, sessionData)
	out = append(out, sig...)
	return out
}

func parseSessionValue(data []byte) (sessionData []byte, sig []byte, _ error) {
	sessionData, sig, err := sbe.ReadLP32(data)
	if err != nil {
		return nil, nil, err
	}
	return sessionData, sig, nil
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
	publicKeyRef, err := src.PublicKeyRef()
	if err != nil {
		return err
	}
	newPublicKeyRef, err := dst.NewPublicKeyRef()
	if err != nil {
		return err
	}
	if err := copyCNPRef(newPublicKeyRef, publicKeyRef); err != nil {
		return err
	}
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
