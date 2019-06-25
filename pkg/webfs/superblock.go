package webfs

import (
	"encoding/json"
	"io/ioutil"
)

type Superblock struct {
	p string
}

func NewSuperblock(p string) *Superblock {
	return &Superblock{p: p}
}

func (s *Superblock) LoadRoot() ([]byte, error) {
	data, err := ioutil.ReadFile(s.p)
	if err != nil {
		return nil, err
	}
	x := struct {
		Root []byte `json:"root"`
	}{}
	if err := json.Unmarshal(data, &x); err != nil {
		return nil, err
	}
	return x.Root, nil
}

func (s *Superblock) StoreRoot(data []byte) error {
	x := struct {
		Root []byte `json:"root"`
	}{}
	x.Root = data
	data, err := json.Marshal(x)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(s.p, data, 0644)
}
