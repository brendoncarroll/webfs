package httpcell

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/brendoncarroll/webfs/pkg/cells"
)

func init() {
	cells.Register(Spec{}, func(x interface{}) cells.Cell {
		return New(x.(Spec))
	})
}

const authHeader = "Authorization"

type Spec struct {
	URL        string
	AuthHeader string
}

type HttpCell struct {
	Spec
	hc http.Client
}

func New(spec Spec) *HttpCell {
	return &HttpCell{
		Spec: spec,
		hc:   http.Client{},
	}
}

func (c *HttpCell) ID() string {
	return "httpcell-" + c.URL
}

func (c *HttpCell) Load(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.Spec.URL, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add(authHeader, c.AuthHeader)
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

func (c *HttpCell) CAS(ctx context.Context, cur, next []byte) (bool, error) {
	creq := CASReq{
		Current: cur,
		Next:    next,
	}
	reqBody, _ := json.Marshal(creq)

	req, err := http.NewRequest(http.MethodPut, c.URL, bytes.NewBuffer(reqBody))
	if err != nil {
		return false, err
	}
	req.Header.Add(authHeader, c.AuthHeader)
	resp, err := c.hc.Do(req)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Println(err)
		}
	}()
	return false, nil
}
