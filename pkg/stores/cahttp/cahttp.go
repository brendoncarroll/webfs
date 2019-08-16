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

const MaxBlobSize = 1 << 16

type CAHttpStore struct {
	endpoint    string
	prefix      string
	maxBlobSize int
}

func New(endpoint string, prefix string) *CAHttpStore {
	if len(endpoint) > 0 && endpoint[len(endpoint)-1] != '/' {
		endpoint += "/"
	}
	return &CAHttpStore{
		endpoint:    endpoint,
		maxBlobSize: MaxBlobSize,
		prefix:      prefix,
	}
}

func (hs *CAHttpStore) Get(ctx context.Context, key string) ([]byte, error) {
	key2, err := hs.removePrefix(key)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(hs.endpoint + key2)
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
	resp, err := http.Post(hs.endpoint, `application/data`, buf)
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
