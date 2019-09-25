package rwacryptocell

import (
	"crypto/rand"
	fmt "fmt"

	"golang.org/x/crypto/nacl/box"
)

func AsymEncrypt(data []byte, recvKey *Key) (*AsymEncMsg, error) {
	var (
		senderKey    *Key
		ctext, nonce []byte
		algo         AsymEncAlgo
	)

	switch x := recvKey.Key.(type) {
	case *Key_Curve25519:
		peerPub := [32]byte{}
		copy(peerPub[:], x.Curve25519)

		nonce2 := [24]byte{}
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, err
		}
		nonce = nonce2[:]

		publicKey, privateKey, err := box.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}

		senderKey = &Key{
			Key: &Key_Curve25519{Curve25519: publicKey[:]},
		}
		ctext = box.Seal(nil, data, &nonce2, &peerPub, privateKey)
		algo = AsymEncAlgo_CURVE25519_NACLBOX

	default:
		return nil, fmt.Errorf("can't encrypt for receiver key %v", recvKey)
	}

	return &AsymEncMsg{
		Algo:      algo,
		SenderKey: senderKey,
		Ctext:     ctext,
	}, nil
}

func AsymDecrypt(privateKey *Key, msg *AsymEncMsg) ([]byte, error) {
	var (
		data []byte
	)

	switch msg.Algo {
	case AsymEncAlgo_CURVE25519_NACLBOX:
		privateKey2, ok := privateKey.Key.(*Key_Curve25519_Private)
		if !ok {
			return nil, fmt.Errorf("wrong private key to decrypt message %v type=%T", msg.Algo, privateKey.Key)
		}
		peersPub, ok := msg.SenderKey.Key.(*Key_Curve25519)
		if !ok {
			return nil, fmt.Errorf("wrong peer public key to decrypt %v", msg.Algo)
		}

		nonce := [24]byte{}
		copy(nonce[:], msg.Nonce)
		privateKey3 := [32]byte{}
		copy(privateKey3[:], privateKey2.Curve25519_Private)
		peersPublicKey := [32]byte{}
		copy(peersPublicKey[:], peersPub.Curve25519)

		var verified bool
		data, verified = box.Open(nil, msg.Ctext, &nonce, &peersPublicKey, &privateKey3)
		if !verified {
			return nil, fmt.Errorf("box is unverified")
		}

	default:
		return nil, fmt.Errorf("algorithm not recognized")
	}
	return data, nil
}
