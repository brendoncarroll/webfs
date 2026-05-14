package webfs

import (
	"context"
	"encoding/binary"
	"fmt"
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

	var value []byte
	exists, err := tx.sessiontx.Get(ctx, id[:], &value)
	if err != nil {
		return inet256.ID{}, err
	}

	var session wfscnp.Session
	if exists {
		sessionData, _, err := parseSessionValue(value)
		if err != nil {
			return inet256.ID{}, err
		}
		msg, err := capnp.Unmarshal(sessionData)
		if err != nil {
			return inet256.ID{}, err
		}
		prevSession, err := wfscnp.ReadRootSession(msg)
		if err != nil {
			return inet256.ID{}, err
		}
		_, seg := capnp.NewSingleSegmentMessage(nil)
		session, err = wfscnp.NewRootSession(seg)
		if err != nil {
			return inet256.ID{}, err
		}
		createdAt, err := prevSession.CreateAt()
		if err != nil {
			return inet256.ID{}, err
		}
		newCreatedAt, err := session.NewCreateAt()
		if err != nil {
			return inet256.ID{}, err
		}
		copyCNPTime(newCreatedAt, createdAt)
		session.SetTtl(prevSession.Ttl())
		publicKeyRef, err := prevSession.PublicKeyRef()
		if err != nil {
			return inet256.ID{}, err
		}
		newPublicKeyRef, err := session.NewPublicKeyRef()
		if err != nil {
			return inet256.ID{}, err
		}
		if err := copyCNPRef(newPublicKeyRef, publicKeyRef); err != nil {
			return inet256.ID{}, err
		}
	} else {
		_, seg := capnp.NewSingleSegmentMessage(nil)
		session, err = wfscnp.NewRootSession(seg)
		if err != nil {
			return inet256.ID{}, err
		}
		createdAt, err := session.NewCreateAt()
		if err != nil {
			return inet256.ID{}, err
		}
		setCNPTime(createdAt, now)
		publicKeyData, err := tx.pki.MarshalPublicKey(nil, publicKey)
		if err != nil {
			return inet256.ID{}, err
		}
		publicKeyRef, err := tx.fdata.Post(ctx, tx.rws, publicKeyData)
		if err != nil {
			return inet256.ID{}, err
		}
		ref, err := session.NewPublicKeyRef()
		if err != nil {
			return inet256.ID{}, err
		}
		if err := setCNPRef(ref, publicKeyRef); err != nil {
			return inet256.ID{}, err
		}
	}

	touchedAt, err := session.NewTouchedAt()
	if err != nil {
		return inet256.ID{}, err
	}
	setCNPTime(touchedAt, now)
	sessionData, err := session.Message().Marshal()
	if err != nil {
		return inet256.ID{}, err
	}
	sig := tx.pki.Sign(&sessionSigCtx, privateKey, sessionSigMessage(tx.gid, sessionData), nil)
	return id, tx.sessiontx.Put(ctx, id[:], makeSessionValue(nil, sessionData, sig))
}

func (tx *Tx) dropSession(ctx context.Context, id inet256.ID) error {
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
			sessionData, _, err := parseSessionValue(buf[i].Value)
			if err != nil {
				return err
			}
			msg, err := capnp.Unmarshal(sessionData)
			if err != nil {
				return err
			}
			session, err := wfscnp.ReadRootSession(msg)
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
	deadline := tai64.TAI64N{
		Seconds:     base.Seconds() + uint64(ttl),
		Nanoseconds: base.Nanoseconds(),
	}
	return !deadline.After(now), nil
}

func sessionTimeBase(session wfscnp.Session) (wfscnp.TAI64N, error) {
	if session.HasTouchedAt() {
		return session.TouchedAt()
	}
	return session.CreateAt()
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
