package memcell

import (
	"testing"

	"github.com/brendoncarroll/webfs/pkg/cells"
)

func TestCell(t *testing.T) {
	cells.CellTestSuite(t, func() cells.Cell {
		return New()
	})
}
