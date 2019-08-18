package webref

import "crypto/rand"

// func MergeOptions(opts []Options) Options {
// 	o := opts[0]
// 	for i := range opts[1:] {
// 		o2 := opts[i]
// 		// crypto
// 		if o2.EncAlgo != EncAlgo_UNKNOWN {
// 			o.EncAlgo = o2.EncAlgo
// 		}
// 		if o2.SecretSeed != nil {
// 			o.SecretSeed = o2.SecretSeed
// 		}
// 		if o2.LengthObfuscation != nil {
// 			o.LengthObfuscation = o2.LengthObfuscation
// 		}

// 		// reppack
// 		if o2.Replicas != nil {
// 			o.Replicas = o2.Replicas
// 		}
// 	}
// 	return o
// }

func DefaultOptions() *Options {
	secret := [32]byte{}
	_, err := rand.Read(secret[:])
	if err != nil {
		panic(err)
	}
	return &Options{
		Attrs: map[string]string{
			"codec": CodecProtobuf,
		},
		EncAlgo:         EncAlgo_CHACHA20,
		SecretSeed:      secret[:],
		ObfuscateLength: false,
		Replicas:        map[string]int32{},
	}
}
