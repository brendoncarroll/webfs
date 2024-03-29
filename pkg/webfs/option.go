package webfs

import (
	"context"
	"net"
	"os"
	"path/filepath"

	bcclient "github.com/blobcache/blobcache/client/go_client"
	"github.com/brendoncarroll/go-state/posixfs"
	ipfsapi "github.com/ipfs/go-ipfs-api"
	"github.com/sirupsen/logrus"
)

type fsConfig struct {
	log               logrus.FieldLogger
	pfs               posixfs.FS
	dialer            TCPDialer
	blobcacheEndpoint string
	ipfs              *ipfsapi.Shell
}

func defaultConfig() fsConfig {
	return fsConfig{
		log:               logrus.StandardLogger(),
		pfs:               posixfs.NewDirFS(filepath.Join(os.TempDir(), "webfs")),
		blobcacheEndpoint: bcclient.DefaultEndpoint,
	}
}

// Option is used to configure a WebFS instance.
type Option func(c *fsConfig)

// WithPosixFS sets x as the filesystem to use for file backed cells.
func WithPosixFS(x posixfs.FS) Option {
	return func(c *fsConfig) {
		c.pfs = x
	}
}

type TCPDialer = func(context.Context, string) (net.Conn, error)

// WithTCPDialer sets the dialer used for outbound TCP connections.
func WithTCPDialer(d func(context.Context, string) (net.Conn, error)) Option {
	return func(c *fsConfig) {
		c.dialer = d
	}
}

// WithBlobcache sets the endpoint to connect to the blobcache daemon
func WithBlobcache(endpoint string) Option {
	return func(c *fsConfig) {
		c.blobcacheEndpoint = endpoint
	}
}

// WithLogger sets the logger for the WebFS instance
func WithLogger(l logrus.FieldLogger) Option {
	return func(c *fsConfig) {
		c.log = l
	}
}

func WithIPFS(shell *ipfsapi.Shell) Option {
	return func(c *fsConfig) {
		c.ipfs = shell
	}
}
