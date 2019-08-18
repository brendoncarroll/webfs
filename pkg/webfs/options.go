package webfs

import (
	"github.com/brendoncarroll/webfs/pkg/stores/ipfsstore"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Options = models.Options

func DefaultOptions() *Options {
	dataOpts := webref.DefaultOptions()
	dataOpts.Replicas["ipfs://"] = 1

	return &Options{
		DataOpts: dataOpts,
		StoreSpecs: []*models.StoreSpec{
			{
				Prefix: "ipfs://",
				Spec: &models.StoreSpec_Ipfs{
					Ipfs: &models.IPFSSpec{
						Endpoint: ipfsstore.DefaultLocalURL,
					},
				},
			},
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
