package webfs

import (
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Options = models.Options

func DefaultOptions() *Options {
	return &Options{
		DataOpts: webref.DefaultOptions(),
	}
}
