package webfs

import (
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Options = models.Options

func DefaultOptions() *Options {
	return &Options{
		DataOpts:   webref.DefaultOptions(),
		StoreSpecs: []*models.StoreSpec{
			// {
			// 	Prefix: "bc://",
			// 	Spec: &models.StoreSpec_Cahttp{
			// 		Cahttp: &models.CAHTTPSpec{
			// 			Endpoint: "http://127.0.0.1:6667/",
			// 			Prefix:   "bc://",
			// 		},
			// 	},
			// },
			// {
			// 	Prefix: "ipfs://",
			// 	Spec: &models.StoreSpec_Ipfs{
			// 		Ipfs: &models.IPFSSpec{
			// 			Endpoint: "http://127.0.0.1:5001/",
			// 		},
			// 	},
			// },
		},
	}
}

// MergeOptions
// from first to last overwrite if the field is set.
func MergeOptions(options ...*Options) *Options {
	ret := DefaultOptions()
	for i := range options {
		o := options[i]
		if len(o.StoreSpecs) > 0 {
			ret.StoreSpecs = o.StoreSpecs
		}
	}
	return ret
}
