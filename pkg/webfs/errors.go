package webfs

import (
	"fmt"
)

// ErrBadConfig is returned when WebFS encounters an invalid config which it cannot mount.
type ErrBadConfig struct {
	Path  string
	Data  []byte
	Inner error
}

func (e ErrBadConfig) Cause() error {
	return e.Inner
}

func (e ErrBadConfig) Error() string {
	return fmt.Sprintf("bad webfs config at path %q. data=%q error=%v", e.Path, e.Data, e.Inner)
}
