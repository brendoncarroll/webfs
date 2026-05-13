package webfs

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"blobcache.io/blobcache/src/bcsdk"
	"blobcache.io/blobcache/src/blobcache"
	"github.com/gotvc/got/src/gdat"
	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/exp/sbe"
	"go.brendoncarroll.net/tai64"
)

// INode is a unique identifier for each object in the filesystem
type INode [16]byte

func newINode() (ret [16]byte) {
	unixnano := time.Now().UnixNano()
	binary.BigEndian.PutUint64(ret[:8], uint64(unixnano))
	return ret
}

func rootINode() (ret [16]byte) {
	return ret
}

func (ino INode) String() string {
	return hex.EncodeToString(ino[:])
}

func OpenRoot(secret blobcache.DEK, ctext []byte) (Root, error) {
	return Root{}, nil
}

func SealRoot(secret blobcache.DEK, root Root, out []byte) []byte {
	return out
}

type Root struct {
	version uint16
	maxSize uint32
	salt    [32]byte
	inodes  gotkv.Root
}

func (r Root) Marshal(out []byte) []byte {
	out = sbe.AppendUint16(out, r.version)
	out = sbe.AppendUint32(out, r.maxSize)
	out = append(out, r.salt[:]...)

	out = append(out, r.inodes.Ref.Marshal()...)
	out = append(out, r.inodes.Depth)
	return out
}

func (r *Root) Unmarshal(data []byte) error {
	version, data, err := sbe.ReadUint16(data)
	if err != nil {
		return err
	}
	r.version = version
	maxSize, _, err := sbe.ReadUint32(data)
	if err != nil {
		return err
	}
	r.maxSize = maxSize
	saltData, data, err := sbe.ReadN(data, 32)
	if err != nil {
		return err
	}
	r.salt = [32]byte(saltData)

	if len(data) != gdat.RefSize+1 {
		return fmt.Errorf("wrong size for fsroot")
	}
	var zero INode
	if err := r.inodes.Unmarshal(data); err != nil {
		return err
	}
	r.inodes.First = zero[:]
	return nil
}

func LoadRoot(ctx context.Context, ldr bcsdk.Loader) (Root, error) {
	var data []byte
	if err := ldr.Load(ctx, &data); err != nil {
		return Root{}, err
	}
	var root Root
	return root, root.Unmarshal(data)
}

type TypeCode uint16

const (
	TC_UNKNOWN = TypeCode(iota)
	TC_REGULAR_FILE
	TC_DIR
	TC_VOLUME_LINK
)

// Node contains information common to all node types.
type Node struct {
	Type TypeCode
	// RefCount is the number of times this Node appears in a DirEnt
	RefCount uint32
	// Rev is the revision, or the number of operations that have been performed on the object
	Rev        uint64
	ModifiedAt tai64.TAI64N

	// Data is the rest of the data in the node
	*File
	*Dir
}

func (n Node) Marshal(out []byte) []byte {
	out = sbe.AppendUint16(out, uint16(n.Type))
	out = sbe.AppendUint32(out, n.RefCount)
	out = sbe.AppendUint64(out, n.Rev)
	switch n.Type {
	case TC_REGULAR_FILE:
		out = n.File.Marshal(out)
	case TC_DIR:
		out = n.Dir.Marshal(out)
	}
	return out
}

func (n *Node) Unmarshal(data []byte) error {
	ty, data, err := sbe.ReadUint16(data)
	if err != nil {
		return err
	}
	n.Type = TypeCode(ty)
	rc, data, err := sbe.ReadUint32(data)
	if err != nil {
		return err
	}
	n.RefCount = rc
	n.Data = data
	return nil
}

// FileInfo attempts to parse the Node data as a FileInfo
func (n *Node) FileInfo() (File, error)

// DirInfo attempts to parse the Node data as a DirInfo
func (n *Node) DirInfo() (Dir, error) {
	return Dir{}, nil
}

type Dir struct {
}

type File struct {
	// Size is the size of the file in bytes
	Size uint64
	// BlockSize is the size of a single block in this file
	BlockSize uint32
}

func (f *File) Marshal(out []byte) []byte {
	out = sbe.AppendUint64(out, f.Size)
	out = sbe.AppendUint32(out, f.BlockSize)
	out = append(out, f.Modtime.Marshal()...)
	return out
}

func (f *File) Unmarshal(data []byte) error {
	size, data, err := sbe.ReadUint64(data)
	if err != nil {
		return err
	}
	f.Size = size
	blockSize, data, err := sbe.ReadUint32(data)
	if err != nil {
		return err
	}
	f.BlockSize = blockSize
	mtData, data, err := sbe.ReadN(data, tai64.TAI64NSize)
	if err != nil {
		return err
	}
	if err := f.Modtime.UnmarshalBinary(mtData); err != nil {
		return err
	}
	return nil
}

type SubVolumeInfo struct {
}
