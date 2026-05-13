package webfs

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"blobcache.io/blobcache/src/bcsdk"
	"github.com/gotvc/got/src/gdat"
	"github.com/gotvc/got/src/gotkv"
	"go.brendoncarroll.net/exp/sbe"
)

// INode is a unique identifier for each object in the filesystem
type INode [16]byte

func newINode() (ret [16]byte) {
	unixnano := time.Now().UnixNano()
	binary.BigEndian.PutUint64(ret[:8], uint64(unixnano))
	rand.Read(ret[8:])
	return ret
}

func rootINode() (ret [16]byte) {
	return ret
}

func (ino INode) String() string {
	return hex.EncodeToString(ino[:])
}

// FSState transitively contains the full state of the filesystem
type FSState struct {
	version     uint16
	maxBlobSize uint32
	salt        [32]byte
	inodes      gotkv.Root
}

func (r FSState) Marshal(out []byte) []byte {
	out = sbe.AppendUint16(out, r.version)
	out = sbe.AppendUint32(out, r.maxBlobSize)
	out = append(out, r.salt[:]...)

	out = append(out, r.inodes.Ref.Marshal()...)
	out = append(out, r.inodes.Depth)
	return out
}

func (r *FSState) Unmarshal(data []byte) error {
	version, data, err := sbe.ReadUint16(data)
	if err != nil {
		return err
	}
	r.version = version
	maxSize, data, err := sbe.ReadUint32(data)
	if err != nil {
		return err
	}
	r.maxBlobSize = maxSize
	saltData, data, err := sbe.ReadN(data, 32)
	if err != nil {
		return err
	}
	r.salt = [32]byte(saltData)

	if len(data) != gdat.RefSize+1 {
		return fmt.Errorf("wrong size for gotkv root HAVE: %d WANT: %d", len(data), gdat.RefSize+1)
	}
	var zero INode
	if err := r.inodes.Unmarshal(data); err != nil {
		return err
	}
	r.inodes.First = zero[:]
	return nil
}

func LoadState(ctx context.Context, ldr bcsdk.Loader) (FSState, error) {
	var data []byte
	if err := ldr.Load(ctx, &data); err != nil {
		return FSState{}, err
	}
	var root FSState
	return root, root.Unmarshal(data)
}

func SaveState(ctx context.Context, svr bcsdk.Saver, root FSState) error {
	return svr.Save(ctx, root.Marshal(nil))
}
