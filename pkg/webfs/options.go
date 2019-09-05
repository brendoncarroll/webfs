package webfs

import (
	"github.com/brendoncarroll/webfs/pkg/stores/ipfsstore"
	"github.com/brendoncarroll/webfs/pkg/webfs/models"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Options = models.Options

func DefaultOptions() *Options {
	dataOpts := webref.DefaultOptions()

	return &Options{
		DataOpts: dataOpts,
		StoreSpecs: []*models.StoreSpec{
			{
				Prefix: "ipfs://",
				Spec: &models.StoreSpec_Ipfs{
					Ipfs: &models.IPFSStoreSpec{
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
		if o == nil {
			continue
		}
		if len(o.StoreSpecs) > 0 {
			ret.StoreSpecs = o.StoreSpecs
		}
		ret.DataOpts = o.DataOpts
	}
	return ret
}
