package secretboxcell

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"sync/atomic"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"golang.org/x/crypto/nacl/secretbox"
)

type Spec struct {
	Inner  cells.Cell
	Secret []byte
}

type payloadMapping struct {
	ptext, ctext []byte
}

type Cell struct {
	spec  Spec
	inner cells.Cell

	lastPayload atomic.Value
}

func New(spec Spec) *Cell {
	c := &Cell{
		spec:  spec,
		inner: spec.Inner,
	}
	c.lastPayload.Store((*payloadMapping)(nil))
	return c
}

func (c *Cell) Get(ctx context.Context) ([]byte, error) {
	ctext, err := c.inner.Get(ctx)
	if err != nil {
		return nil, err
	}
	if len(ctext) < 1 {
		return nil, nil
	}

	var (
		nonce = [24]byte{}
		key   = [32]byte{}
	)

	if len(ctext) < len(nonce) {
		return nil, errors.New("ciphertext too short")
	}
	copy(nonce[:], ctext[:])
	box := ctext[len(nonce):]

	copy(key[:], c.spec.Secret[:])
	ptext, valid := secretbox.Open(nil, box, &nonce, &key)
	if !valid {
		return nil, errors.New("invalid ciphertext")
	}

	c.lastPayload.Store(&payloadMapping{
		ctext: ctext,
		ptext: ptext,
	})

	return ptext, nil
}

func (c *Cell) CAS(ctx context.Context, current, next []byte) (bool, error) {
	pm := c.lastPayload.Load().(*payloadMapping)
	if pm != nil && bytes.Compare(pm.ptext, current) != 0 {
		return false, nil
	}
	if pm == nil {
		pm = &payloadMapping{}
	}

	var (
		nonce = [24]byte{}
		key   = [32]byte{}
	)
	if _, err := rand.Read(nonce[:]); err != nil {
		return false, err
	}
	copy(key[:], c.spec.Secret)

	payload := make([]byte, len(nonce))
	copy(payload, nonce[:])
	payload = secretbox.Seal(payload, next, &nonce, &key)

	success, err := c.inner.CAS(ctx, pm.ctext, payload)
	if err != nil {
		return false, err
	}
	if success {
		c.lastPayload.Store(&payloadMapping{
			ctext: payload,
			ptext: next,
		})
	}
	return success, nil
}

func (c *Cell) URL() string {
	return c.inner.URL()
}
