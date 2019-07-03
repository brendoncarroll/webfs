package webfs

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// Superblock On Disk Format
type SBODF struct {
}

type Superblock struct {
	p string
}

func NewSuperblock(p string) (*Superblock, error) {
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	sb := &Superblock{p: p}
	return sb, sb.StoreRoot(nil)
}

func SuperblockFromPath(p string) (*Superblock, error) {
	sb := &Superblock{p: p}
	_, err := sb.LoadRoot()
	return sb, err
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
	data, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(s.p, data, 0644)
}
