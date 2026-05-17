package webfs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"

	capnp "capnproto.org/go/capnp/v3"
	"github.com/brendoncarroll/webfs/src/internal/wfscnp"
	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/exp/sbe"
	"go.brendoncarroll.net/exp/streams"
	"go.brendoncarroll.net/tai64"
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
	Owner     uint64
	State     wfscnp.LockState
}

var ErrLockConflict = errors.New("lock conflict")

func (tx *FSTx) EnsureSession(ctx context.Context, now tai64.TAI64N) (inet256.ID, error) {
	if tx.priv == nil {
		return inet256.ID{}, fmt.Errorf("tx has no private key")
	}
	return tx.ensureSession(ctx, tx.priv, now)
}

func (tx *FSTx) GetLock(ctx context.Context, ino INode, sessionID inet256.ID, owner uint64) (wfscnp.LockState, error) {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.getLock(ctx, ino, sessionID, owner)
}

func (tx *FSTx) SetLock(ctx context.Context, sessionID inet256.ID, owner uint64, ino INode, kind LockKind, start, length uint64) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if kind == 0 {
		return tx.removeLock(ctx, sessionID, owner, ino)
	}
	return tx.addLock(ctx, sessionID, owner, ino, kind, start, length)
}

func (tx *FSTx) FindConflictingLock(ctx context.Context, ino INode, sessionID inet256.ID, owner uint64, kind LockKind, start, length uint64) (*LockInfo, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	return tx.findConflictingLockLocked(ctx, ino, sessionID, owner, kind, start, length, true)
}

// addLock assumes the write lock is held
func (tx *FSTx) addLock(ctx context.Context, sessionID inet256.ID, owner uint64, ino INode, kind LockKind, start, length uint64) error {
	if _, err := tx.getSession(ctx, sessionID); err != nil {
		return err
	}
	if _, err := tx.getLock(ctx, ino, sessionID, owner); err == nil {
		return fmt.Errorf("%w: lock already held by session %v owner %d on inode %v", ErrLockConflict, sessionID, owner, ino)
	} else if !fsErrNotExist(err) {
		return err
	}
	conflict, err := tx.findConflictingLockLocked(ctx, ino, sessionID, owner, kind, start, length, true)
	if err != nil {
		return err
	}
	if conflict != nil {
		return fmt.Errorf("%w with session %v owner %d on inode %v", ErrLockConflict, conflict.SessionID, conflict.Owner, ino)
	}
	state, err := newLockState(sessionID, kind, start, length)
	if err != nil {
		return err
	}
	if err := tx.putLock(ctx, ino, sessionID, owner, state); err != nil {
		return err
	}
	return tx.incrementSessionLockCount(ctx, sessionID, 1)
}

// removeLock assumes the lock is held
func (tx *FSTx) removeLock(ctx context.Context, sessionID inet256.ID, owner uint64, ino INode) error {
	if _, err := tx.getLock(ctx, ino, sessionID, owner); err != nil {
		return err
	}
	if err := tx.locktx.Delete(ctx, makeLockKey(nil, ino, sessionID, owner)); err != nil {
		return err
	}
	return tx.incrementSessionLockCount(ctx, sessionID, -1)
}

// getLock assumes the lock is held
func (tx *FSTx) getLock(ctx context.Context, ino INode, sessionID inet256.ID, owner uint64) (wfscnp.LockState, error) {
	var value []byte
	exists, err := tx.locktx.Get(ctx, makeLockKey(nil, ino, sessionID, owner), &value)
	if err != nil {
		return wfscnp.LockState{}, err
	}
	if !exists {
		return wfscnp.LockState{}, fs.ErrNotExist
	}
	lockData, pubKey, err := openSigned(ctx, tx.pkcache, &lockSigCtx, tx.ros, value)
	if err != nil {
		return wfscnp.LockState{}, err
	}
	if tx.sys.pki.NewID(pubKey) != sessionID {
		return wfscnp.LockState{}, fmt.Errorf("lock has wrong signer for session %v", sessionID)
	}
	return parseLock(lockData)
}

func (tx *FSTx) putLock(ctx context.Context, ino INode, sessionID inet256.ID, owner uint64, state wfscnp.LockState) error {
	lockData, err := state.Message().Marshal()
	if err != nil {
		return err
	}
	privKey, err := tx.getSessionPrivateKey(ctx, sessionID)
	if err != nil {
		return err
	}
	val, err := sealSigned(ctx, tx.pkcache, &lockSigCtx, tx.rws, privKey, lockData, nil)
	if err != nil {
		return err
	}
	return tx.locktx.Put(ctx, makeLockKey(nil, ino, sessionID, owner), val)
}

// deleteLocksForSession assumes the lock is held
func (tx *FSTx) deleteLocksForSession(ctx context.Context, sessionID inet256.ID) error {
	if tx.locktx.Queued() > 0 {
		if _, err := tx.locktx.Flush(ctx); err != nil {
			return err
		}
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
			ino, holder, _, ok := parseLockKey(buf[i].Key)
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

func (tx *FSTx) findConflictingLockLocked(ctx context.Context, ino INode, sessionID inet256.ID, owner uint64, kind LockKind, start, length uint64, haveWriteLock bool) (*LockInfo, error) {
	locks, err := tx.listLocks(ctx, ino, haveWriteLock)
	if err != nil {
		return nil, err
	}
	for i := range locks {
		if locks[i].SessionID == sessionID && locks[i].Owner == owner {
			continue
		}
		otherKind := LockKind(locks[i].State.Kind())
		if !locksConflict(kind, start, length, otherKind, locks[i].State.Start(), locks[i].State.Length()) {
			continue
		}
		return &locks[i], nil
	}
	return nil, nil
}

func (tx *FSTx) listLocks(ctx context.Context, ino INode, haveWriteLock bool) ([]LockInfo, error) {
	if tx.locktx.Queued() > 0 {
		if !haveWriteLock {
			return nil, fmt.Errorf("listLocks needs to flush writes, but does not have the write lock")
		}
		if _, err := tx.locktx.Flush(ctx); err != nil {
			return nil, err
		}
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
			ino2, sessionID, owner, ok := parseLockKey(buf[i].Key)
			if !ok || ino2 != ino {
				continue
			}
			lock, err := tx.getLock(ctx, ino, sessionID, owner)
			if err != nil {
				return nil, err
			}
			ret = append(ret, LockInfo{INode: ino, SessionID: sessionID, Owner: owner, State: lock})
		}
	}
}

func makeLockKey(out []byte, ino INode, sessionID inet256.ID, owner uint64) []byte {
	out = append(out, ino[:]...)
	out = append(out, sessionID[:]...)
	out = sbe.AppendUint64(out, owner)
	return out
}

func parseLockKey(key []byte) (INode, inet256.ID, uint64, bool) {
	if len(key) != len(INode{})+len(inet256.ID{})+8 {
		return INode{}, inet256.ID{}, 0, false
	}
	var ino INode
	copy(ino[:], key[:len(ino)])
	var sessionID inet256.ID
	copy(sessionID[:], key[len(ino):])
	owner, _, err := sbe.ReadUint64(key[len(ino)+len(sessionID):])
	if err != nil {
		return INode{}, inet256.ID{}, 0, false
	}
	return ino, sessionID, owner, true
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
	out = append(out, lockData...)
	out = append(out, sig...)
	return out
}

func parseLock(data []byte) (wfscnp.LockState, error) {
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return wfscnp.LockState{}, err
	}
	lstate, err := wfscnp.ReadRootLockState(msg)
	if err != nil {
		return wfscnp.LockState{}, err
	}
	return lstate, nil
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
