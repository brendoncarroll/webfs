package webfs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
)

type CellSpec struct {
	EncAlgo string
	Secret  []byte
}

func (spec *CellSpec) ID() string {
	return "cell-a"
}

func NewCell(spec CellSpec) Cell {
	return nil
}

type Cell interface {
	ID() string
	Load(ctx context.Context) ([]byte, error)
}

type CASCell interface {
	Cell
	CAS(ctx context.Context, cur, next []byte) (bool, error)
}

type CellMutator func(Object) (*Object, error)

func Apply(ctx context.Context, cell Cell, f CellMutator) error {
	wcell, ok := cell.(CASCell)
	if !ok {
		return errors.New("cell is not writeable")
	}

	const maxRetries = 10
	success := false
	for i := 0; !success && i < maxRetries; i++ {
		current, err := wcell.Load(ctx)
		if err != nil {
			return err
		}

		currentO := Object{}
		if err := json.Unmarshal(current, &currentO); err != nil {
			return err
		}

		nextO, err := f(currentO)
		if err != nil {
			return err
		}
		next, err := json.Marshal(nextO)
		if err != nil {
			return err
		}
		success, err = wcell.CAS(ctx, current, next)
		if err != nil {
			return err
		}
	}

	if !success {
		return errors.New("could not complete CAS")
	}

	return nil
}

type RootCell struct {
	mu         sync.Mutex
	data       []byte
	superblock *Superblock
}

func (rc *RootCell) ID() string {
	return ""
}

func (rc *RootCell) Load(ctx context.Context) ([]byte, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	data, err := rc.superblock.LoadRoot()
	if err != nil {
		return nil, err
	}
	rc.data = data
	return data, nil
}

func (rc *RootCell) CAS(ctx context.Context, cur []byte, next []byte) (bool, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if bytes.Compare(rc.data, cur) != 0 {
		return false, nil
	}
	if err := rc.superblock.StoreRoot(next); err != nil {
		return false, err
	}
	rc.data = next
	return true, nil
}

// type UnionNamespace struct {
// 	Layers []Object
// }

// func (ns *UnionNamespace) Get(ctx context.Context, k []byte) ([]byte, error) {
// 	for _, ns2 := range ns.Layers {
// 		v, err := ns2.Get(ctx, k)
// 		if err != nil {
// 			return nil, err
// 		}
// 		if v != nil {
// 			return v, nil
// 		}
// 	}
// 	return nil, nil
// }
