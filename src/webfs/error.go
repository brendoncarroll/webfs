package webfs

import (
	"fmt"
)

type ErrWrongType struct {
	INode
	WantType string
}

func (e *ErrWrongType) Error() string {
	return fmt.Sprintf("%v is not type %v", e.INode, e.WantType)
}
