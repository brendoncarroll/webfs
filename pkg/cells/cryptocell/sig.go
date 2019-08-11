package cryptocell

import (
	"crypto/sha256"
	"errors"
	fmt "fmt"
	"hash"

	"golang.org/x/crypto/ed25519"
)

func VerifySig(data []byte, signer *Key, sig *Sig) error {
	var (
		h        hash.Hash
		verified bool
	)
	if sig == nil {
		return errors.New("nil Sig is not a valid Sig")
	}

	switch sig.HashFunc {
	case HashFunc_SHA256:
		h = sha256.New()
	default:
		return fmt.Errorf("hash function not supported: %v", sig.HashFunc)
	}
	digest := h.Sum(data)

	switch sig.Algo {
	case SigAlgo_ED25519:
		signer2, ok := signer.Key.(*Key_Ed25519)
		if !ok {
			return fmt.Errorf("wrong key %v for algo %v", signer, sig.Algo)
		}

		publicKey := signer2.Ed25519
		verified = ed25519.Verify(publicKey, digest, sig.Sig)
	default:
		return fmt.Errorf("signature algorithm not supported: %v", sig.Algo)
	}

	if !verified {
		return fmt.Errorf("signature is invalid")
	}
	return nil
}

func Sign(data []byte, signer *Key) (*Sig, error) {
	var (
		algo     = SigAlgo_ED25519
		hashFunc = HashFunc_SHA256
		h        hash.Hash
		sigBytes []byte
	)

	switch hashFunc {
	case HashFunc_SHA256:
		h = sha256.New()
	}
	digest := h.Sum(data)

	switch algo {
	case SigAlgo_ED25519:
		signer2 := signer.Key.(*Key_Ed25519_Private)
		sigBytes = ed25519.Sign(signer2.Ed25519_Private, digest)
	default:
		return nil, fmt.Errorf("signature algorithm not supported: %v", algo)
	}

	sig := &Sig{
		Algo:     algo,
		HashFunc: hashFunc,
		Sig:      sigBytes,
	}
	return sig, nil
}
