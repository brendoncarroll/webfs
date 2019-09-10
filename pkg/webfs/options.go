package webfs

import (
	"crypto/rand"

	"github.com/brendoncarroll/webfs/pkg/stores/ipfsstore"
	"github.com/brendoncarroll/webfs/pkg/webfsim"
	"github.com/brendoncarroll/webfs/pkg/webref"
)

type Options = webfsim.Options

func DefaultWriteOptions() webfsim.WriteOptions {
	return webfsim.WriteOptions{
		Codec:           webref.CodecProtobuf,
		EncAlgo:         webref.EncAlgo_CHACHA20,
		SecretSeed:      randomBytes(32),
		ObfuscateLength: false,
		Replicas:        map[string]int32{},
	}
}

func randomBytes(l int) []byte {
	secret := [32]byte{}
	_, err := rand.Read(secret[:])
	if err != nil {
		panic(err)
	}
	return secret[:]
}

func DefaultOptions() *Options {
	dataOpts := DefaultWriteOptions()

	return &Options{
		DataOpts: &dataOpts,
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
