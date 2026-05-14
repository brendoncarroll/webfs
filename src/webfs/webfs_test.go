package webfs

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"blobcache.io/blobcache/src/bclocal"
	"blobcache.io/blobcache/src/blobcache"
	"github.com/brendoncarroll/webfs/src/internal/wfscnp"
	"github.com/stretchr/testify/require"
	"go.brendoncarroll.net/tai64"
	"go.inet256.org/inet256/src/inet256"
)

func TestFile(t *testing.T) {
	ctx := context.Background()
	sys, vcfg := setupVol(t)
	var fileIno INode
	contents := "hello world"
	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		ino, err := tx.CreateFileAt(ctx, INode{}, "a", 0o755, FileParams{
			Now:       tai64.Now(),
			BlockSize: 4096,
		})
		require.NoError(t, err)
		if err := tx.WriteAt(ctx, ino, 0, []byte(contents)); err != nil {
			return err
		}
		fileIno = ino
		return nil
	}))
	require.NoError(t, sys.View(ctx, vcfg, func(tx *Tx) error {
		buf := make([]byte, 100)
		n, err := tx.ReadAt(ctx, fileIno, 0, buf)
		actual := string(buf[:n])
		require.Equal(t, contents, actual)
		return err
	}))
}

func TestDir(t *testing.T) {
	ctx := context.Background()
	sys, vcfg := setupVol(t)
	contents := []byte("hello world")
	var fileIno INode

	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		ino, err := tx.CreateFileAt(ctx, rootINode(), "a", 0o644, FileParams{
			Now:       tai64.Now(),
			BlockSize: 4096,
		})
		if err != nil {
			return err
		}
		fileIno = ino
		if err := tx.WriteAt(ctx, fileIno, 0, contents); err != nil {
			return err
		}
		if err := tx.Link(ctx, rootINode(), "b", fileIno); err != nil {
			return err
		}

		a, err := tx.GetChild(ctx, rootINode(), "a")
		if err != nil {
			return err
		}
		b, err := tx.GetChild(ctx, rootINode(), "b")
		if err != nil {
			return err
		}
		require.Equal(t, a.Target, b.Target)

		if err := tx.Unlink(ctx, rootINode(), "a"); err != nil {
			return err
		}
		if _, err := tx.GetChild(ctx, rootINode(), "a"); err == nil || !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("expected a to be missing after unlink, err=%v", err)
		}

		buf := make([]byte, len(contents))
		n, err := tx.ReadAt(ctx, fileIno, 0, buf)
		if err != nil {
			return err
		}
		require.Equal(t, len(contents), n)
		require.Equal(t, string(contents), string(buf[:n]))

		if err := tx.Unlink(ctx, rootINode(), "b"); err != nil {
			return err
		}
		if _, err := tx.GetChild(ctx, rootINode(), "b"); err == nil || !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("expected b to be missing after unlink, err=%v", err)
		}
		if _, err := tx.ReadAt(ctx, fileIno, 0, make([]byte, 1)); err == nil {
			return fmt.Errorf("expected file inode to be deleted after final unlink")
		}
		return nil
	}))
}

func TestXAttrs(t *testing.T) {
	ctx := context.Background()
	sys, vcfg := setupVol(t)
	var fileIno INode

	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		ino, err := tx.CreateFileAt(ctx, rootINode(), "a", 0o644, FileParams{
			Now:       tai64.Now(),
			BlockSize: 4096,
		})
		require.NoError(t, err)
		fileIno = ino

		require.NoError(t, tx.SetXAttr(ctx, fileIno, "user.key", []byte("old")))
		require.NoError(t, tx.SetXAttr(ctx, fileIno, "user.key", []byte("new")))
		return nil
	}))

	require.NoError(t, sys.View(ctx, vcfg, func(tx *Tx) error {
		value, err := tx.GetXAttr(ctx, fileIno, "user.key")
		require.NoError(t, err)
		require.Equal(t, []byte("new"), value)
		return nil
	}))

	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		require.NoError(t, tx.RemoveXAttr(ctx, fileIno, "user.key"))
		_, err := tx.GetXAttr(ctx, fileIno, "user.key")
		require.ErrorIs(t, err, fs.ErrNotExist)
		return nil
	}))
}

func TestXAttrSizeLimits(t *testing.T) {
	ctx := context.Background()
	sys, vcfg := setupVol(t)

	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		ino, err := tx.CreateFileAt(ctx, rootINode(), "a", 0o644, FileParams{
			Now:       tai64.Now(),
			BlockSize: 4096,
		})
		require.NoError(t, err)

		require.NoError(t, tx.SetXAttr(ctx, ino, XAttrKey(strings.Repeat("a", MaxXAttrKeySize)), nil))
		require.Error(t, tx.SetXAttr(ctx, ino, XAttrKey(strings.Repeat("a", MaxXAttrKeySize+1)), nil))
		require.NoError(t, tx.SetXAttr(ctx, ino, "user.value", make([]byte, MaxXAttrValueSize)))
		require.Error(t, tx.SetXAttr(ctx, ino, "user.value", make([]byte, MaxXAttrValueSize+1)))
		return nil
	}))
}

func TestSessions(t *testing.T) {
	ctx := context.Background()
	sys, vcfg := setupVol(t)
	require.NotEqual(t, [32]byte{}, vcfg.GID)
	pubKey, privKey, err := sys.pki.GenerateKey()
	require.NoError(t, err)
	wantID := sys.pki.NewID(pubKey)
	createdAt := tai64.TAI64N{Seconds: 1, Nanoseconds: 2}
	touchedAt := tai64.TAI64N{Seconds: 3, Nanoseconds: 4}

	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		id, err := tx.ensureSession(ctx, privKey, createdAt)
		require.NoError(t, err)
		require.Equal(t, wantID, id)
		require.Equal(t, vcfg.GID, tx.gid)
		require.Equal(t, tx.gid, tx.prev.gid)

		session := requireSessionValue(t, ctx, sys, tx, pubKey, wantID)
		require.Equal(t, createdAt, readCNPTime(t, session.CreateAt))
		require.Equal(t, createdAt, readCNPTime(t, session.TouchedAt))
		require.True(t, session.HasPublicKeyRef())

		id, err = tx.ensureSession(ctx, privKey, touchedAt)
		require.NoError(t, err)
		require.Equal(t, wantID, id)

		session = requireSessionValue(t, ctx, sys, tx, pubKey, wantID)
		require.Equal(t, createdAt, readCNPTime(t, session.CreateAt))
		require.Equal(t, touchedAt, readCNPTime(t, session.TouchedAt))
		return nil
	}))

	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		tx.gid[0] ^= 0xff
		_, err := tx.ensureSession(ctx, privKey, tai64.Now())
		require.Error(t, err)
		require.Contains(t, err.Error(), "gid")
		return nil
	}))

	require.NoError(t, sys.View(ctx, vcfg, func(tx *Tx) error {
		require.Equal(t, vcfg.GID, tx.gid)
		requireSessionValue(t, ctx, sys, tx, pubKey, wantID)
		return nil
	}))

	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		require.NoError(t, tx.dropSession(ctx, wantID))
		var value []byte
		exists, err := tx.sessiontx.Get(ctx, wantID[:], &value)
		require.NoError(t, err)
		require.False(t, exists)
		return nil
	}))
}

func TestGCSessions(t *testing.T) {
	ctx := context.Background()
	sys, vcfg := setupVol(t)
	_, expiredPriv, err := sys.pki.GenerateKey()
	require.NoError(t, err)
	expiredID := sys.pki.NewID(inet256.PublicFromPrivate(expiredPriv))
	_, activePriv, err := sys.pki.GenerateKey()
	require.NoError(t, err)
	activeID := sys.pki.NewID(inet256.PublicFromPrivate(activePriv))
	_, immortalPriv, err := sys.pki.GenerateKey()
	require.NoError(t, err)
	immortalID := sys.pki.NewID(inet256.PublicFromPrivate(immortalPriv))

	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		require.NoError(t, putTestSession(ctx, tx, expiredPriv, tai64.TAI64N{Seconds: 1}, 1))
		require.NoError(t, putTestSession(ctx, tx, activePriv, tai64.Now(), 3600))
		require.NoError(t, putTestSession(ctx, tx, immortalPriv, tai64.TAI64N{Seconds: 1}, 0))

		require.NoError(t, tx.GCSessions(ctx))
		requireSessionMissing(t, ctx, tx, expiredID)
		requireSessionExists(t, ctx, tx, activeID)
		requireSessionExists(t, ctx, tx, immortalID)
		return nil
	}))
}

func TestLocks(t *testing.T) {
	ctx := context.Background()
	sys, vcfg := setupVol(t)
	privKeyData, err := hex.DecodeString(vcfg.PrivateKeyHex)
	require.NoError(t, err)
	privKey1, err := sys.pki.ParsePrivateKey(privKeyData)
	require.NoError(t, err)
	pubKey1 := inet256.PublicFromPrivate(privKey1)
	id1 := sys.pki.NewID(pubKey1)
	var fileIno INode

	require.NoError(t, sys.Modify(ctx, vcfg, func(tx *Tx) error {
		ino, err := tx.CreateFileAt(ctx, rootINode(), "locked", 0o644, FileParams{
			Now:       tai64.Now(),
			BlockSize: 4096,
		})
		require.NoError(t, err)
		fileIno = ino

		_, err = tx.ensureSession(ctx, privKey1, tai64.Now())
		require.NoError(t, err)
		require.NoError(t, tx.addLock(ctx, id1, fileIno, LockKindRead, 0, 100))
		lock1, err := tx.getLock(ctx, fileIno, id1)
		require.NoError(t, err)
		require.Equal(t, uint16(LockKindRead), lock1.Kind())
		require.Equal(t, uint64(0), lock1.Start())
		require.Equal(t, uint64(100), lock1.Length())
		require.Equal(t, uint32(1), requireSessionValue(t, ctx, sys, tx, pubKey1, id1).LockCount())

		err = tx.addLock(ctx, id1, fileIno, LockKindRead, 0, 100)
		require.Error(t, err)

		err = tx.addLock(ctx, id1, fileIno, LockKindWrite, 50, 100)
		require.Error(t, err)

		require.NoError(t, tx.removeLock(ctx, id1, fileIno))
		require.Equal(t, uint32(0), requireSessionValue(t, ctx, sys, tx, pubKey1, id1).LockCount())
		_, err = tx.getLock(ctx, fileIno, id1)
		require.ErrorIs(t, err, fs.ErrNotExist)
		require.NoError(t, tx.addLock(ctx, id1, fileIno, LockKindWrite, 0, 0))
		require.Equal(t, uint32(1), requireSessionValue(t, ctx, sys, tx, pubKey1, id1).LockCount())
		require.NoError(t, tx.dropSession(ctx, id1))
		requireSessionMissing(t, ctx, tx, id1)
		_, err = tx.getLock(ctx, fileIno, id1)
		require.ErrorIs(t, err, fs.ErrNotExist)
		return nil
	}))
}

func requireSessionValue(t testing.TB, ctx context.Context, sys *System, tx *Tx, pubKey inet256.PublicKey, id inet256.ID) wfscnp.Session {
	t.Helper()
	session, err := tx.getSession(ctx, id)
	require.NoError(t, err)
	require.Equal(t, id, sys.pki.NewID(pubKey))
	return session
}

func readCNPTime(t testing.TB, get func() (wfscnp.TAI64N, error)) tai64.TAI64N {
	t.Helper()
	actual, err := get()
	require.NoError(t, err)
	return tai64.TAI64N{Seconds: actual.Seconds(), Nanoseconds: actual.Nanoseconds()}
}

func putTestSession(ctx context.Context, tx *Tx, privKey inet256.PrivateKey, touchedAt tai64.TAI64N, ttl uint32) error {
	id, err := tx.ensureSession(ctx, privKey, touchedAt)
	if err != nil {
		return err
	}
	_, err = tx.editSession(ctx, id, privKey, func(session wfscnp.Session) error {
		createAt, err := session.CreateAt()
		if err != nil {
			return err
		}
		setCNPTime(createAt, touchedAt)
		touchedAtCNP, err := session.TouchedAt()
		if err != nil {
			return err
		}
		setCNPTime(touchedAtCNP, touchedAt)
		session.SetTtl(ttl)
		return nil
	})
	return err
}

func requireSessionExists(t testing.TB, ctx context.Context, tx *Tx, id inet256.ID) {
	t.Helper()
	var value []byte
	exists, err := tx.sessiontx.Get(ctx, id[:], &value)
	require.NoError(t, err)
	require.True(t, exists)
}

func requireSessionMissing(t testing.TB, ctx context.Context, tx *Tx, id inet256.ID) {
	t.Helper()
	var value []byte
	exists, err := tx.sessiontx.Get(ctx, id[:], &value)
	require.NoError(t, err)
	require.False(t, exists)
}

func setupVol(t testing.TB) (*System, VolumeConfig) {
	ctx := context.Background()
	bc := bclocal.NewTestService(t)
	volh, err := bc.CreateVolume(ctx, nil, blobcache.VolumeSpec{
		Local: &blobcache.VolumeBackend_Local{
			HashAlgo: blobcache.HashAlgo_BLAKE2b_256,
			MaxSize:  1 << 21,
		},
	})
	require.NoError(t, err)
	sys := NewSystem(bc)
	vcfg, err := sys.Initialize(ctx, *volh)
	require.NoError(t, err)
	return sys, vcfg
}
