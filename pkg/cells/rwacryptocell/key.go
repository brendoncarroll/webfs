package rwacryptocell

import (
	"crypto/rand"

	"github.com/golang/protobuf/proto"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/nacl/box"
)

func GenerateEntity() (*Entity, error) {
	_, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	signingKey := &Key{
		Key: &Key_Ed25519_Private{Ed25519_Private: edPriv},
	}

	_, cryPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	encryptKey := &Key{
		Key: &Key_Curve25519_Private{Curve25519_Private: cryPriv[:]},
	}

	return &Entity{
		SigningKey:    signingKey,
		EncryptionKey: encryptKey,
	}, nil
}

func GetPublicEntity(privEnt *Entity) *Entity {
	return &Entity{
		SigningKey:    GetPublicKey(privEnt.SigningKey),
		EncryptionKey: GetPublicKey(privEnt.EncryptionKey),
	}
}

func GetPublicKey(key *Key) *Key {
	switch x := key.Key.(type) {
	case *Key_Curve25519_Private:
		privateKey := new([32]byte)
		copy(privateKey[:], x.Curve25519_Private)
		publicKey := new([32]byte)
		curve25519.ScalarBaseMult(publicKey, privateKey)
		return &Key{
			Key: &Key_Curve25519{Curve25519: publicKey[:]},
		}

	case *Key_Ed25519_Private:
		const publicOffsetInPrivate = 32
		publicKey := make([]byte, ed25519.PublicKeySize)
		copy(publicKey, x.Ed25519_Private[publicOffsetInPrivate:])
		return &Key{
			Key: &Key_Ed25519{Ed25519: publicKey},
		}
	}
	return nil
}

func findEntity(es []*Entity, e *Entity) int {
	for i := range es {
		if proto.Equal(es[i], e) {
			return i
		}
	}
	return -1
}
