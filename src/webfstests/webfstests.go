package webfstests

import (
	"testing"

	"blobcache.io/blobcache/src/bclocal"
	"github.com/brendoncarroll/webfs/src/webfs"
)

func New(t testing.TB) *webfs.System {
	bc := bclocal.NewTestService(t)
	pki := webfs.DefaultPKI()
	return webfs.NewSystem(bc, pki)
}
