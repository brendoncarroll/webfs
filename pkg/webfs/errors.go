package webfs

import (
	"errors"
	"os"
)

var (
	ErrNotExist = os.ErrNotExist

	ErrObjectType        = errors.New("invalid object type")
	ErrDuplicateVolumeID = errors.New("duplicate volume id")

	ErrConcurrentMod = errors.New("the object was removed or replaced with an object of another type")
)
