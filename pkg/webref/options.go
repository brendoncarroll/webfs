package webref

import (
	"crypto/rand"
)

func DefaultOptions() *Options {
	return &Options{
		Attrs: map[string]string{
			"codec": CodecProtobuf,
		},
		EncAlgo:         EncAlgo_CHACHA20,
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
