package webfs

import (
	"context"
	"net"
	"os"
	"path/filepath"

	bcclient "github.com/blobcache/blobcache/client/go_client"
	"github.com/brendoncarroll/go-state/posixfs"
	"github.com/sirupsen/logrus"
)

type TCPDialer = func(context.Context, string) (net.Conn, error)

type fsConfig struct {
	log               logrus.FieldLogger
	pfs               posixfs.FS
	dialer            TCPDialer
	blobcacheEndpoint string
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

// WithPOSIXFS sets x as the filesystem to use for file backed cells.
func WithPOSIXFS(x posixfs.FS) Option {
	return func(c *fsConfig) {
		c.pfs = x
	}
}

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
