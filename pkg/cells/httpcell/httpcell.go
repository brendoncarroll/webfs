package httpcell

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"golang.org/x/crypto/sha3"
)

const (
	currentHeader = "X-Current"
)

type Spec struct {
	URL     string
	Headers map[string]string
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
	req := c.newRequest(ctx, http.MethodGet, c.spec.URL, nil)

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

	req := c.newRequest(ctx, http.MethodPut, c.spec.URL, bytes.NewBuffer(next))
	req.Header.Set(currentHeader, curHashb64)

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

func (c *Cell) newRequest(ctx context.Context, method, u string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		panic(err)
	}
	req = req.WithContext(ctx)

	for k, v := range c.spec.Headers {
		req.Header.Set(k, v)
	}
	return req
}
