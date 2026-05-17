package webfs

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

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
	gid         GID
	salt        [32]byte
	inodes      gotkv.Root
	exts        gotkv.Root
	dirEnts     gotkv.Root
	xattrs      gotkv.Root
	sessions    gotkv.Root
	locks       gotkv.Root
}

func (r FSState) Marshal(out []byte) []byte {
	out = sbe.AppendUint16(out, r.version)
	out = sbe.AppendUint32(out, r.maxBlobSize)
	out = append(out, r.gid[:]...)
	out = append(out, r.salt[:]...)

	out = appendRoot(out, r.inodes)
	out = appendRoot(out, r.exts)
	out = appendRoot(out, r.dirEnts)
	out = appendRoot(out, r.xattrs)
	out = appendRoot(out, r.sessions)
	out = appendRoot(out, r.locks)
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
	gidData, data, err := sbe.ReadN(data, 32)
	if err != nil {
		return err
	}
	r.gid = [32]byte(gidData)
	saltData, data, err := sbe.ReadN(data, 32)
	if err != nil {
		return err
	}
	r.salt = [32]byte(saltData)

	inodes, data, err := readRoot(data)
	if err != nil {
		return err
	}
	r.inodes = inodes
	exts, data, err := readRoot(data)
	if err != nil {
		return err
	}
	r.exts = exts
	dirEnts, data, err := readRoot(data)
	if err != nil {
		return err
	}
	r.dirEnts = dirEnts
	xattrs, data, err := readRoot(data)
	if err != nil {
		return err
	}
	r.xattrs = xattrs
	sessions, data, err := readRoot(data)
	if err != nil {
		return err
	}
	r.sessions = sessions
	locks, data, err := readRoot(data)
	if err != nil {
		return err
	}
	r.locks = locks
	if len(data) != 0 {
		return fmt.Errorf("unexpected trailing state data: %d bytes", len(data))
	}
	return nil
}

func appendRoot(out []byte, root gotkv.Root) []byte {
	return sbe.AppendLP(out, root.Marshal(nil))
}

func readRoot(data []byte) (gotkv.Root, []byte, error) {
	rootData, rest, err := sbe.ReadLP(data)
	if err != nil {
		return gotkv.Root{}, nil, err
	}
	var root gotkv.Root
	if err := root.Unmarshal(rootData); err != nil {
		return gotkv.Root{}, nil, err
	}
	return root, rest, nil
}
