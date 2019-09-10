package webfs

import (
	"os"
	"time"
)

type FileInfo struct {
	Mode       os.FileMode
	ModifiedAt time.Time
	CreatedAt  time.Time
	AccessedAt time.Time
	Size       uint64
}
