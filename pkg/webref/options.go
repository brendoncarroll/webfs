package webref

type Options struct {
	EncAlgo    string
	SecretSeed []byte

	Replicas map[string]int
}
