package rwacryptocell

import (
	"crypto/cipher"
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

func SymDecrypt(msg *EncMsg, key []byte) ([]byte, error) {
	var (
		ciph  cipher.AEAD
		err   error
		algo  = msg.Algo
		ctext = msg.Ctext
		nonce = msg.Nonce
	)
	switch algo {
	case EncAlgo_XCHACHA20POLY1305:
		ciph, err = chacha20poly1305.NewX(key)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported algo: %v", algo)
	}
	return ciph.Open(nil, nonce, ctext, nil)
}

func SymEncrypt(ptext []byte) (*EncMsg, []byte, error) {
	var (
		ciph  cipher.AEAD
		err   error
		key   []byte
		nonce []byte
	)

	algo := EncAlgo_XCHACHA20POLY1305 // just support one for now
	switch algo {
	case EncAlgo_XCHACHA20POLY1305:
		key = make([]byte, 32)
		if _, err = rand.Read(key[:]); err != nil {
			return nil, nil, err
		}
		nonce = make([]byte, chacha20poly1305.NonceSizeX)
		if _, err = rand.Read(nonce[:]); err != nil {
			return nil, nil, err
		}
		ciph, err = chacha20poly1305.NewX(key)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("unsupported algo: %v", algo)
	}

	ctext := ciph.Seal(nil, nonce, ptext, nil)

	return &EncMsg{
		Algo:  algo,
		Nonce: nonce,
		Ctext: ctext,
	}, key, nil
}
