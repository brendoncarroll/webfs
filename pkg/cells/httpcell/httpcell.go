package httpcell

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/brendoncarroll/webfs/pkg/cells"
	"golang.org/x/crypto/sha3"
)

func init() {
	cells.Register(Spec{}, func(x interface{}) cells.Cell {
		return New(x.(Spec))
	})
}

const (
	authHeader    = "Authorization"
	currentHeader = "X-Current"
)

type Spec struct {
	URL        string
	AuthHeader string
}

type Cell struct {
	spec Spec
	hc   *http.Client
}

func New(spec Spec) *Cell {
	return &Cell{
		spec: spec,
		hc:   http.DefaultClient,
	}
}

func (c *Cell) URL() string {
	return c.spec.URL
}

func (c *Cell) Get(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.spec.URL, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add(authHeader, c.spec.AuthHeader)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Println(err)
		}
	}()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Cell) CAS(ctx context.Context, cur, next []byte) (bool, error) {
	curHash := sha3.Sum256(cur)
	curHashb64 := base64.URLEncoding.EncodeToString(curHash[:])
	req, err := http.NewRequest(http.MethodPut, c.spec.URL, bytes.NewBuffer(next))
	if err != nil {
		return false, err
	}
	req.Header.Add(authHeader, c.spec.AuthHeader)
	req.Header.Add(currentHeader, curHashb64)
	resp, err := c.hc.Do(req)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Println(err)
		}
	}()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	success := bytes.Compare(next, data) == 0
	return success, nil
}
