package webfs

import (
	"github.com/brendoncarroll/webfs/pkg/stores/ipfsstore"
	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Options = webfsim.Options

func DefaultOptions() *Options {
	dataOpts := webref.DefaultOptions()

	return &Options{
		DataOpts: dataOpts,
		StoreSpecs: []*webfsim.StoreSpec{
			{
				Prefix: "ipfs://",
				Spec: &webfsim.StoreSpec_Ipfs{
					Ipfs: &webfsim.IPFSStoreSpec{
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
