package secretboxcell

import (
	"testing"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"github.com/brendoncarroll/webfs/pkg/cells/memcell"
	"golang.org/x/crypto/sha3"
)

func TestCell(t *testing.T) {
	cells.CellTestSuite(t, func() cells.Cell {
		secret := sha3.Sum256([]byte("my secret key which will hash to exactly 32 bytes"))
		spec := Spec{
			Inner:  memcell.New(),
			Secret: secret[:],
		}

		return New(spec)
	})
}
