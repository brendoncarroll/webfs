package webfs

import (
	"errors"
	"os"
)

var (
	ErrNotExist = os.ErrNotExist

	ErrObjectType        = errors.New("invalid object type")
	ErrDuplicateVolumeID = errors.New("duplicate volume id")
)
