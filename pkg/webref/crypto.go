package webref

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"

	"golang.org/x/crypto/sha3"

	// golang.org/x/crypto/internal/chacha20 is internal
	"github.com/Yawning/chacha20"
	"github.com/brendoncarroll/webfs/pkg/stores"
)

type CryptoRef struct {
	URL     string `json:"url"`
	EncAlgo string `json:"enc-algo"`
	Secret  []byte `json:"secret"`
	Length  uint64 `json:"length"`
}

func PostCrypto(ctx context.Context, s stores.WriteOnce, prefix string, data []byte, o Options) (*CryptoRef, error) {
	secret := generateSecret(data, o.SecretSeed)

	ctext := make([]byte, len(data))
	if err := crypt(o.EncAlgo, secret, data, ctext); err != nil {
		return nil, err
	}

	key, err := s.Post(ctx, prefix, ctext)
	if err != nil {
		return nil, err
	}
	return &CryptoRef{
		EncAlgo: o.EncAlgo,
		Secret:  secret,
		URL:     string(key),
		Length:  uint64(len(data)),
	}, nil
}

func (r *CryptoRef) Deref(ctx context.Context, store stores.Read) ([]byte, error) {
	payload, err := store.Get(ctx, r.URL)
	if err != nil {
		return nil, err
	}
	if err := crypt(r.EncAlgo, r.Secret, payload, payload); err != nil {
		return nil, err
	}

	return payload[:r.Length], nil
}

func crypt(algo string, secret, in, out []byte) error {
	switch algo {
	case "aes-256-ctr":
		iv := [16]byte{} // 0
		blockCipher, err := aes.NewCipher(secret)
		if err != nil {
			return err
		}
		streamCipher := cipher.NewCTR(blockCipher, iv[:])
		streamCipher.XORKeyStream(out, in)
	case "chacha20":
		nonce := [chacha20.NonceSize]byte{} // 0
		streamCipher, err := chacha20.NewCipher(secret, nonce[:])
		if err != nil {
			return err
		}
		streamCipher.XORKeyStream(out, in)
	default:
		return errors.New("unsupported enc-algo")
	}
	return nil
}

// secrets are generated from a seed and the
// data to be encrypted.
// - an empty seed is totally convergent
// - any other seed is convergent with other data
//   encrypted with the same seed
func generateSecret(seed, data []byte) []byte {
	d := sha3.Sum256(data)

	// concat hash of data with seed
	x := bytes.Join([][]byte{
		seed,
		d[:],
	}, nil)

	y := sha3.Sum256(x)
	return y[:]
}
