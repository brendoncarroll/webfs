package webfs

import (
	"fmt"

	"go.inet256.org/inet256/src/inet256"
)

type ErrWrongType struct {
	INode
	WantType string
}

func (e *ErrWrongType) Error() string {
	return fmt.Sprintf("%v is not type %v", e.INode, e.WantType)
}

type ErrNotAllowed struct {
	Actor inet256.ID
	Op    string
}

func (e *ErrNotAllowed) Error() string {
	return fmt.Sprintf("%v is not allowed to %v", e.Actor, e.Op)
}
