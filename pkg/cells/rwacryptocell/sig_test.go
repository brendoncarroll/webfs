package rwacryptocell

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSig(t *testing.T) {
	privateEntity, err := GenerateEntity()
	require.Nil(t, err)

	publicEntity := GetPublicEntity(privateEntity)

	buf := make([]byte, 1024)
	_, err = rand.Read(buf)
	if err != nil {
		panic(err)
	}
	sig, err := Sign(buf, privateEntity.SigningKey)
	require.Nil(t, err)

	err = VerifySig(buf, publicEntity.SigningKey, sig)
	require.Nil(t, err)
}
