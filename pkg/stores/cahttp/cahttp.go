package cahttp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

type CAHttpStore struct {
	u           string
	prefix      string
	maxBlobSize int
}

func NewCAHttp(u string, prefix string) *CAHttpStore {
	return &CAHttpStore{
		u:           u,
		maxBlobSize: 1 << 16,
		prefix:      prefix,
	}
}

func (hs *CAHttpStore) Get(ctx context.Context, key string) ([]byte, error) {
	key2, err := hs.removePrefix(key)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(hs.u + "/" + key2)
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

func (hs *CAHttpStore) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	_, err := hs.removePrefix(prefix)
	if err != nil {
		return "", err
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

	idStr := base64.URLEncoding.EncodeToString(idBytes)
	key := hs.prefix + idStr
	return key, nil
}

func (hs *CAHttpStore) MaxBlobSize() int {
	return hs.maxBlobSize
}

func (hs *CAHttpStore) removePrefix(x string) (string, error) {
	if !strings.HasPrefix(x, hs.prefix) {
		return "", errors.New("Wrong key: " + x)
	}
	y := x[len(hs.prefix):]
	return y, nil
}
