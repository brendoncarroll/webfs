package webref

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	fmt "fmt"
	"math/bits"

	"golang.org/x/crypto/sha3"

	// golang.org/x/crypto/internal/chacha20 is internal
	"github.com/Yawning/chacha20"
	"github.com/brendoncarroll/webfs/pkg/stores"
)

func PostCrypto(ctx context.Context, s stores.Post, prefix string, data []byte, o Options) (*CryptoRef, error) {
	secret := generateSecret(data, o.SecretSeed)

	ctextLen := len(data)
	if o.ObfuscateLength {
		ctextLen = obfuscatedLength(len(data))
	}

	ctext := make([]byte, ctextLen)
	if err := crypt(o.EncAlgo, secret, data, ctext); err != nil {
		return nil, err
	}

	if _, err := rand.Read(ctext[len(data):]); err != nil {
		return nil, err
	}

	key, err := s.Post(ctx, prefix, ctext)
	if err != nil {
		return nil, err
	}
	return &CryptoRef{
		EncAlgo: o.EncAlgo,
		Dek:     secret,
		Url:     string(key),
		Length:  int32(len(data)),
	}, nil
}

func GetCrypto(ctx context.Context, store stores.Read, r *CryptoRef) ([]byte, error) {
	payload, err := store.Get(ctx, r.Url)
	if err != nil {
		return nil, err
	}
	if len(payload) < int(r.Length) {
		return nil, fmt.Errorf("broken ref length is wrong have: %d want: %d", len(payload), r.Length)
	}
	if err := crypt(r.EncAlgo, r.Dek, payload, payload); err != nil {
		return nil, err
	}
	return payload[:r.Length], nil
}

func crypt(algo EncAlgo, secret, in, out []byte) error {
	switch algo {
	case EncAlgo_AES256CTR:
		iv := [16]byte{} // 0
		blockCipher, err := aes.NewCipher(secret)
		if err != nil {
			return err
		}
		streamCipher := cipher.NewCTR(blockCipher, iv[:])
		streamCipher.XORKeyStream(out, in)
	case EncAlgo_CHACHA20:
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

func DeleteCrypto(ctx context.Context, s stores.Delete, r *CryptoRef) error {
	return s.Delete(ctx, r.Url)
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

func obfuscatedLength(x int) int {
	if x == 0 {
		return 0
	}
	l := bits.Len(uint(x))
	if bits.OnesCount(uint(x)) == 1 {
		l--
	}
	return 1 << uint(l)
}
