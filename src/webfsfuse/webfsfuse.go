package webfsfuse

import (
	"context"
	"syscall"

	"github.com/brendoncarroll/webfs/src/webfs"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Node types must be InodeEmbedders
var _ = (fs.InodeEmbedder)((*Node)(nil))

// Node types should implement some file system operations, eg. Lookup
var _ = (fs.NodeLookuper)((*Node)(nil))

type Node struct {
	fs.Inode
	sys     *webfs.System
	rootCfg webfs.VolumeConfig
}

func NewRoot(sys *webfs.System, rootCfg webfs.VolumeConfig) *Node {
	return &Node{
		sys:     sys,
		rootCfg: rootCfg,
	}
}

func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	ops := Node{}
	out.Mode = 0755
	out.Size = 42
	return n.NewInode(ctx, &ops, fs.StableAttr{Mode: syscall.S_IFREG}), 0
}
