package webfs

import (
	"context"
	"fmt"
	"io/fs"
	"slices"

	capnp "capnproto.org/go/capnp/v3"
	"github.com/brendoncarroll/webfs/src/internal/wfscnp"
	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/exp/sbe"
	"go.brendoncarroll.net/exp/streams"
	"go.inet256.org/inet256/src/inet256"
)

var lockSigCtx = inet256.SigCtxString("webfs/lock")

type LockKind uint16

const (
	LockKindRead LockKind = 1 + iota
	LockKindWrite
)

type LockInfo struct {
	INode     INode
	SessionID inet256.ID
	State     wfscnp.LockState
}

func (tx *Tx) addLock(ctx context.Context, sessionID inet256.ID, ino INode, kind LockKind, start, length uint64) error {
	if _, err := tx.getSession(ctx, sessionID); err != nil {
		return err
	}
	if _, err := tx.getLock(ctx, ino, sessionID); err == nil {
		return fmt.Errorf("lock already held by session %v on inode %v", sessionID, ino)
	} else if !fsErrNotExist(err) {
		return err
	}
	conflict, err := tx.findConflictingLock(ctx, ino, kind, start, length)
	if err != nil {
		return err
	}
	if conflict != nil {
		return fmt.Errorf("lock conflicts with session %v on inode %v", conflict.SessionID, ino)
	}
	state, err := newLockState(sessionID, kind, start, length)
	if err != nil {
		return err
	}
	if err := tx.putLock(ctx, ino, sessionID, state); err != nil {
		return err
	}
	return tx.incrementSessionLockCount(ctx, sessionID, 1)
}

func (tx *Tx) removeLock(ctx context.Context, sessionID inet256.ID, ino INode) error {
	if _, err := tx.getLock(ctx, ino, sessionID); err != nil {
		return err
	}
	if err := tx.locktx.Delete(ctx, makeLockKey(nil, ino, sessionID)); err != nil {
		return err
	}
	return tx.incrementSessionLockCount(ctx, sessionID, -1)
}

func (tx *Tx) getLock(ctx context.Context, ino INode, sessionID inet256.ID) (wfscnp.LockState, error) {
	var value []byte
	exists, err := tx.locktx.Get(ctx, makeLockKey(nil, ino, sessionID), &value)
	if err != nil {
		return wfscnp.LockState{}, err
	}
	if !exists {
		return wfscnp.LockState{}, fs.ErrNotExist
	}
	return tx.unmarshalLockValue(ino, sessionID, value)
}

func (tx *Tx) putLock(ctx context.Context, ino INode, sessionID inet256.ID, state wfscnp.LockState) error {
	lockData, err := state.Message().Marshal()
	if err != nil {
		return err
	}
	lockPriv, err := tx.privateKeyForSession(ctx, sessionID)
	if err != nil {
		return err
	}
	lockValue := makeLockValue(nil, lockData, tx.pki.Sign(&lockSigCtx, lockPriv, lockSigMessage(tx.gid, ino, sessionID, lockData), nil))
	return tx.locktx.Put(ctx, makeLockKey(nil, ino, sessionID), lockValue)
}

func (tx *Tx) unmarshalLockValue(ino INode, sessionID inet256.ID, value []byte) (wfscnp.LockState, error) {
	lockData, sig, err := parseLockValue(value)
	if err != nil {
		return wfscnp.LockState{}, err
	}
	session, err := tx.getSession(context.Background(), sessionID)
	if err != nil {
		return wfscnp.LockState{}, err
	}
	pubKey, err := tx.publicKeyFromSession(sessionID, session)
	if err != nil {
		return wfscnp.LockState{}, err
	}
	if !tx.pki.Verify(&lockSigCtx, pubKey, lockSigMessage(tx.gid, ino, sessionID, lockData), sig) {
		return wfscnp.LockState{}, fmt.Errorf("invalid lock signature for inode %v session %v", ino, sessionID)
	}
	msg, err := capnp.Unmarshal(lockData)
	if err != nil {
		return wfscnp.LockState{}, err
	}
	return wfscnp.ReadRootLockState(msg)
}

func (tx *Tx) deleteLocksForSession(ctx context.Context, sessionID inet256.ID) error {
	if _, err := tx.locktx.Flush(ctx); err != nil {
		return err
	}
	it := tx.locktx.IterateFlushed(ctx, gotkv.TotalSpan())
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
			ino, holder, ok := parseLockKey(buf[i].Key)
			if !ok || holder != sessionID {
				continue
			}
			if err := tx.locktx.Delete(ctx, slices.Clone(buf[i].Key)); err != nil {
				return err
			}
			_ = ino
		}
	}
}

func (tx *Tx) incrementSessionLockCount(ctx context.Context, sessionID inet256.ID, delta int32) error {
	_, err := tx.editSessionByID(ctx, sessionID, func(session wfscnp.Session) error {
		next := int64(session.LockCount()) + int64(delta)
		if next < 0 {
			return fmt.Errorf("session lock count underflow for %v", sessionID)
		}
		session.SetLockCount(uint32(next))
		return nil
	})
	return err
}

func (tx *Tx) findConflictingLock(ctx context.Context, ino INode, kind LockKind, start, length uint64) (*LockInfo, error) {
	locks, err := tx.listLocks(ctx, ino)
	if err != nil {
		return nil, err
	}
	for i := range locks {
		otherKind := LockKind(locks[i].State.Kind())
		if !locksConflict(kind, start, length, otherKind, locks[i].State.Start(), locks[i].State.Length()) {
			continue
		}
		return &locks[i], nil
	}
	return nil, nil
}

func (tx *Tx) listLocks(ctx context.Context, ino INode) ([]LockInfo, error) {
	if _, err := tx.locktx.Flush(ctx); err != nil {
		return nil, err
	}
	it := tx.locktx.IterateFlushed(ctx, gotkv.PrefixSpan(ino[:]))
	buf := make([]gotkv.Entry, 32)
	var ret []LockInfo
	for {
		n, err := it.Next(ctx, buf)
		if err != nil {
			if streams.IsEOS(err) {
				return ret, nil
			}
			return nil, err
		}
		for i := 0; i < n; i++ {
			ino2, sessionID, ok := parseLockKey(buf[i].Key)
			if !ok || ino2 != ino {
				continue
			}
			state, err := tx.unmarshalLockValue(ino, sessionID, buf[i].Value)
			if err != nil {
				return nil, err
			}
			ret = append(ret, LockInfo{INode: ino, SessionID: sessionID, State: state})
		}
	}
}

func makeLockKey(out []byte, ino INode, sessionID inet256.ID) []byte {
	out = append(out, ino[:]...)
	out = append(out, sessionID[:]...)
	return out
}

func parseLockKey(key []byte) (INode, inet256.ID, bool) {
	if len(key) != len(INode{})+len(inet256.ID{}) {
		return INode{}, inet256.ID{}, false
	}
	var ino INode
	copy(ino[:], key[:len(ino)])
	var sessionID inet256.ID
	copy(sessionID[:], key[len(ino):])
	return ino, sessionID, true
}

func newLockState(holder inet256.ID, kind LockKind, start, length uint64) (wfscnp.LockState, error) {
	_, seg := capnp.NewSingleSegmentMessage(nil)
	state, err := wfscnp.NewRootLockState(seg)
	if err != nil {
		return wfscnp.LockState{}, err
	}
	if err := state.SetHolder(holder[:]); err != nil {
		return wfscnp.LockState{}, err
	}
	state.SetKind(uint16(kind))
	state.SetStart(start)
	state.SetLength(length)
	return state, nil
}

func makeLockValue(out []byte, lockData []byte, sig []byte) []byte {
	out = sbe.AppendLP32(out, lockData)
	out = append(out, sig...)
	return out
}

func parseLockValue(data []byte) (lockData []byte, sig []byte, _ error) {
	lockData, sig, err := sbe.ReadLP32(data)
	if err != nil {
		return nil, nil, err
	}
	return lockData, sig, nil
}

func lockSigMessage(gid [32]byte, ino INode, sessionID inet256.ID, lockData []byte) []byte {
	msg := make([]byte, 0, len(gid)+len(ino)+len(sessionID)+len(lockData))
	msg = append(msg, gid[:]...)
	msg = append(msg, ino[:]...)
	msg = append(msg, sessionID[:]...)
	msg = append(msg, lockData...)
	return msg
}

func locksConflict(kindA LockKind, startA, lengthA uint64, kindB LockKind, startB, lengthB uint64) bool {
	if kindA == LockKindRead && kindB == LockKindRead {
		return false
	}
	return rangesOverlap(startA, lengthA, startB, lengthB)
}

func rangesOverlap(startA, lengthA, startB, lengthB uint64) bool {
	endA, okA := lockRangeEnd(startA, lengthA)
	endB, okB := lockRangeEnd(startB, lengthB)
	if okA && endA <= startB {
		return false
	}
	if okB && endB <= startA {
		return false
	}
	return true
}

func lockRangeEnd(start, length uint64) (uint64, bool) {
	if length == 0 {
		return 0, false
	}
	return start + length, true
}

func fsErrNotExist(err error) bool {
	return err == fs.ErrNotExist
}
