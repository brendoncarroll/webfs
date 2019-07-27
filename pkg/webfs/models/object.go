package models

import (
	"encoding/json"
	"time"

	"github.com/brendoncarroll/webfs/pkg/cells/httpcell"
	"github.com/brendoncarroll/webfs/pkg/webref"
	"github.com/brendoncarroll/webfs/pkg/wrds"
)

type Options = webref.Options

type Object struct {
	File     *File     `json:"file,omitempty"`
	Dir      *Dir      `json:"dir,omitempty"`
	Cell     *CellSpec `json:"cell,omitempty"`
	Snapshot *Snapshot `json:"snapshot,omitempty"`
}

func (o Object) Marshal() []byte {
	data, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return data
}

func (o *Object) Unmarshal(data []byte) error {
	return json.Unmarshal(data, o)
}

type File struct {
	Checksum []byte `json:"checksum"`
	Size     uint64 `json:"size"`

	Tree *wrds.Tree `json:"tree"`
}

type Snapshot struct {
	Cell      CellSpec  `json:"cell"`
	Contents  Volume    `json:"contents"`
	Timestamp time.Time `json:"timestamp"`
}

type Dir struct {
	Tree *wrds.Tree `json:"tree"`
}

type DirEntry struct {
	Name   string `json:"name"`
	Object Object `json:"object"`
}

type CellSpec struct {
	HTTPCell *httpcell.Spec
	// Do not add FileCell or MemCell here
}

type Volume struct {
	ObjectRef webref.Ref `json:"object_ref"`
	Options   Options    `json:"options"`
}

func (v *Volume) Marshal() []byte {
	data, _ := json.Marshal(v)
	return data
}

func (v *Volume) Unmarshal(data []byte) error {
	return json.Unmarshal(data, v)
}
