package stores

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
)

type CAHttpStore struct {
	u           string
	maxBlobSize int
}

func NewCAHttp(u string) *CAHttpStore {
	return &CAHttpStore{u: u, maxBlobSize: 1 << 16}
}

func (hs *CAHttpStore) Get(ctx context.Context, key string) ([]byte, error) {
	resp, err := http.Get(hs.u + "/" + key)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Println(err)
		}
	}()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (hs *CAHttpStore) Check(ctx context.Context, key string) (bool, error) {
	// TODO: http HEAD
	return false, nil
}

func (hs *CAHttpStore) Post(ctx context.Context, key string, data []byte) (string, error) {
	if len(key) > 0 {
		return "", errors.New("CAHttp does not allow prefixes")
	}
	buf := bytes.NewBuffer(data)
	resp, err := http.Post(hs.u, `application/data`, buf)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Println(err)
		}
	}()

	idBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(idBytes), nil
}

func (hs *CAHttpStore) MaxBlobSize() int {
	return hs.maxBlobSize
}
