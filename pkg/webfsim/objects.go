package webfsim

import (
	"context"
	fmt "fmt"
	"time"

	"github.com/brendoncarroll/webfs/pkg/stores"
	webref "github.com/brendoncarroll/webfs/pkg/webref"
)

type Snapshot struct {
	Volume    VolumeSpec `json:"volume"`
	Commit    Commit     `json:"contents"`
	Timestamp time.Time  `json:"timestamp"`
}

func ObjectSplit(ctx context.Context, s stores.ReadPost, opts webref.Options, o Object) (*Object, error) {
	var (
		o2 *Object
	)
	switch x := o.Value.(type) {
	case *Object_Dir:
		d2, err := DirSplit(ctx, s, opts, *x.Dir)
		if err != nil {
			return nil, err
		}
		o2 = &Object{
			Value: &Object_Dir{Dir: d2},
		}
	case *Object_File:
		f2, err := FileSplit(ctx, s, opts, *x.File)
		if err != nil {
			return nil, err
		}
		o2 = &Object{
			Value: &Object_File{File: f2},
		}
	default:
		return nil, fmt.Errorf("unsplittable object %v", o)
	}

	return o2, nil
}
