package webfs

import (
	"encoding/json"
	"fmt"
)

type Object struct {
	File     *File     `json:"file,omitempty"`
	Dir      *Dir      `json:"dir,omitempty"`
	Cell     *CellSpec `json:"cell,omitempty"`
	Snapshot *Snapshot `json:"snapshot,omitempty"`
}

func (o Object) Size() uint64 {
	switch {
	case o.File != nil:
		return o.File.Size
	default:
		return 0
	}
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

func (o Object) String() string {
	inner := ""
	switch {
	case o.File != nil:
		inner = "File"
	case o.Dir != nil:
		inner = "Dir"
	case o.Cell != nil:
		inner = "Cell"
	}
	return fmt.Sprintf("Object{%s}", inner)
}
