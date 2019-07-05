package webref

import "crypto/rand"

type Options struct {
	EncAlgo           string
	SecretSeed        []byte
	LengthObfuscation *bool

	Replicas map[string]int
}

func MergeOptions(opts []Options) Options {
	o := opts[0]
	for i := range opts[1:] {
		o2 := opts[i]
		// crypto
		if o2.EncAlgo != "" {
			o.EncAlgo = o2.EncAlgo
		}
		if o2.SecretSeed != nil {
			o.SecretSeed = o2.SecretSeed
		}
		if o2.LengthObfuscation != nil {
			o.LengthObfuscation = o2.LengthObfuscation
		}

		// reppack
		if o2.Replicas != nil {
			o.Replicas = o2.Replicas
		}
	}
	return o
}

func DefaultOptions() Options {
	secret := [32]byte{}
	_, err := rand.Read(secret[:])
	if err != nil {
		panic(err)
	}
	lo := false
	return Options{
		EncAlgo:           "chacha20",
		SecretSeed:        secret[:],
		LengthObfuscation: &lo,
		Replicas: map[string]int{
			"bc://": 1,
		},
	}
}
