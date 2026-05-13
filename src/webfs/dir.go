package webfs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"regexp"

	capnp "capnproto.org/go/capnp/v3"
	"github.com/brendoncarroll/webfs/src/internal/wfscnp"
	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/exp/sbe"
	"go.brendoncarroll.net/exp/streams"
	"go.brendoncarroll.net/tai64"
)

type DirEnt struct {
	Name   string
	Mode   fs.FileMode
	Target INode
}

func (tx *Tx) getDir(ctx context.Context, ino INode) (wfscnp.Dir, error) {
	node, err := tx.getNode(ctx, ino)
	if err != nil {
		return wfscnp.Dir{}, err
	}
	if node.Payload().Which() != wfscnp.Node_payload_Which_dir {
		return wfscnp.Dir{}, &ErrWrongType{INode: ino, WantType: "directory"}
	}
	return node.Payload().Dir()
}

// CreateDirAt creates a new empty directory, and adds it to the parent.
func (tx *Tx) CreateDirAt(ctx context.Context, parent INode, name string) (INode, error) {
	ino, err := tx.createDir(ctx)
	if err != nil {
		return INode{}, err
	}
	if err := tx.Link(ctx, parent, name, fs.ModeDir|0o755, ino); err != nil {
		return INode{}, err
	}
	return ino, nil
}

func (tx *Tx) createDir(ctx context.Context) (INode, error) {
	now := tai64.Now()
	_, seg := capnp.NewSingleSegmentMessage(nil)
	node, err := wfscnp.NewRootNode(seg)
	if err != nil {
		return INode{}, err
	}
	node.SetRefCount(0)
	node.SetRev(0)
	createdAt, err := node.NewCreatedAt()
	if err != nil {
		return INode{}, err
	}
	createdAt.SetSeconds(now.Seconds)
	createdAt.SetNanoseconds(now.Nanoseconds)
	modifiedAt, err := node.NewModifiedAt()
	if err != nil {
		return INode{}, err
	}
	modifiedAt.SetSeconds(now.Seconds)
	modifiedAt.SetNanoseconds(now.Nanoseconds)
	if _, err := node.Payload().NewDir(); err != nil {
		return INode{}, err
	}
	ino := newINode()
	if err := tx.putNode(ctx, ino, node); err != nil {
		return INode{}, err
	}
	return ino, nil
}

func (tx *Tx) Link(ctx context.Context, ino INode, name string, mode fs.FileMode, child INode) error {
	if err := checkName(name); err != nil {
		return err
	}
	if _, err := tx.getDir(ctx, ino); err != nil {
		return err
	}
	childNode, err := tx.getNode(ctx, child)
	if err != nil {
		return err
	}
	if _, err := tx.GetChild(ctx, ino, name); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	ent := dirEntValue{Mode: mode, Target: child}
	key := makeDirEntKey(nil, ino, name)
	if err := tx.inodetx.Put(ctx, key, ent.Marshal(nil)); err != nil {
		return err
	}
	childNode.SetRefCount(childNode.RefCount() + 1)
	return tx.putNode(ctx, child, childNode)
}

func (tx *Tx) Unlink(ctx context.Context, ino INode, name string) error {
	if _, err := tx.getDir(ctx, ino); err != nil {
		return err
	}
	ent, err := tx.GetChild(ctx, ino, name)
	if err != nil {
		return err
	}
	childNode, err := tx.getNode(ctx, ent.Target)
	if err != nil {
		return err
	}
	if childNode.RefCount() == 0 {
		return fmt.Errorf("inode %v has invalid refcount 0", ent.Target)
	}
	key := makeDirEntKey(nil, ino, name)
	if err := tx.inodetx.Delete(ctx, key); err != nil {
		return err
	}
	newRefCount := childNode.RefCount() - 1
	if newRefCount > 0 {
		childNode.SetRefCount(newRefCount)
		return tx.putNode(ctx, ent.Target, childNode)
	}
	if _, err := tx.inodetx.Flush(ctx); err != nil {
		return err
	}
	it := tx.inodetx.IterateFlushed(ctx, gotkv.PrefixSpan(ent.Target[:]))
	buf := make([]gotkv.Entry, 32)
	for {
		n, err := it.Next(ctx, buf)
		if err != nil {
			if streams.IsEOS(err) {
				break
			}
			return err
		}
		for i := 0; i < n; i++ {
			if err := tx.inodetx.Delete(ctx, buf[i].Key); err != nil {
				return err
			}
		}
	}
	if tx.inodeCache != nil {
		delete(tx.inodeCache, ent.Target)
	}
	return nil
}

func (tx *Tx) ReadDir(ctx context.Context, ino INode, gteq string) iter.Seq2[DirEnt, error] {
	return func(yield func(DirEnt, error) bool) {
		if _, err := tx.getDir(ctx, ino); err != nil {
			yield(DirEnt{}, err)
			return
		}
		if _, err := tx.inodetx.Flush(ctx); err != nil {
			yield(DirEnt{}, err)
			return
		}
		begin := makeDirEntKey(nil, ino, gteq)
		end := gotkv.PrefixEnd(ino[:])
		it := tx.inodetx.IterateFlushed(ctx, gotkv.Span{Begin: begin, End: end})
		buf := make([]gotkv.Entry, 32)
		for {
			n, err := it.Next(ctx, buf)
			if err != nil {
				if streams.IsEOS(err) {
					return
				}
				yield(DirEnt{}, err)
				return
			}
			for i := 0; i < n; i++ {
				dv, err := parseDirValue(buf[i].Value)
				if err != nil {
					if !yield(DirEnt{}, err) {
						return
					}
					continue
				}
				name := string(buf[i].Key[len(ino):])
				ent := DirEnt{Name: name, Mode: dv.Mode, Target: dv.Target}
				if !yield(ent, nil) {
					return
				}
			}
		}
	}
}

// GetChild looks for an entry in the directory at ino with name and returns it.
// If it does not exist than an ErrNotExist is returned
func (tx *Tx) GetChild(ctx context.Context, ino INode, name string) (DirEnt, error) {
	if _, err := tx.getDir(ctx, ino); err != nil {
		return DirEnt{}, err
	}
	key := makeDirEntKey(nil, ino, name)
	var val []byte
	exists, err := tx.inodetx.Get(ctx, key, &val)
	if err != nil {
		return DirEnt{}, err
	}
	if !exists {
		return DirEnt{}, fs.ErrNotExist
	}
	dv, err := parseDirValue(val)
	if err != nil {
		return DirEnt{}, err
	}
	return DirEnt{
		Name:   name,
		Mode:   dv.Mode,
		Target: dv.Target,
	}, nil
}

func makeDirEntKey(out []byte, ino INode, name string) []byte {
	out = append(out, ino[:]...)
	out = append(out, name...)
	return out
}

var validNameRe = regexp.MustCompile(`^[^/\x00]+$`)

func checkName(name string) error {
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("invalid name %q", name)
	}
	return nil
}

// dirEntValue is the value component of a directory entry
type dirEntValue struct {
	Mode   fs.FileMode
	Target INode
}

func (de dirEntValue) Marshal(out []byte) []byte {
	out = sbe.AppendUint32(out, uint32(de.Mode))
	out = append(out, de.Target[:]...)
	return out
}

func parseDirValue(data []byte) (dirEntValue, error) {
	mode, data, err := sbe.ReadUint32(data)
	if err != nil {
		return dirEntValue{}, err
	}
	targetData, data, err := sbe.ReadN(data, len(INode{}))
	if err != nil {
		return dirEntValue{}, err
	}
	if len(data) > 0 {
		return dirEntValue{}, fmt.Errorf("extra trailing bytes in dirValue: %d", len(data))
	}
	var target INode
	copy(target[:], targetData)
	return dirEntValue{
		Mode:   fs.FileMode(mode),
		Target: target,
	}, nil
}
