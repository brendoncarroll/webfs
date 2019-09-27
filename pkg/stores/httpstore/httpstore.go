package httpstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/multiformats/go-multihash"
)

const MaxBlobSize = 1 << 16

type HttpStore struct {
	endpoint    string
	prefix      string
	maxBlobSize int
}

func New(endpoint string, prefix string) *HttpStore {
	return &HttpStore{
		endpoint:    endpoint,
		maxBlobSize: MaxBlobSize,
		prefix:      prefix,
	}
}

func (hs *HttpStore) Get(ctx context.Context, key string) ([]byte, error) {
	key2, err := hs.removePrefix(key)
	if err != nil {
		return nil, err
	}
	if key2[0] == '/' {
		key2 = key2[1:]
	}

	mhBytes, err := base64.URLEncoding.DecodeString(key2)
	if err != nil {
		return nil, err
	}
	want, err := multihash.Decode(mhBytes)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(hs.getURL(key2))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Println(err)
		}
	}()
	if resp.StatusCode == http.StatusNotFound {
		return nil, stores.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got non-200 status: %s", resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	actual, err := multihash.Sum(data, want.Code, want.Length)
	if err != nil {
		return nil, err
	}
	if bytes.Compare(mhBytes, actual) != 0 {
		return nil, errors.New("got bad data from store")
	}
	return data, nil
}

func (hs *HttpStore) Check(ctx context.Context, key string) (err error) {
	key, err = hs.removePrefix(key)
	if err != nil {
		return err
	}
	u := hs.getURL(key)
	resp, err := http.DefaultClient.Head(u)
	if err != nil {
		return err
	}
	ok := resp.StatusCode == http.StatusOK
	if !ok {
		return errors.New("status: " + resp.Status)
	}
	return nil
}

func (hs *HttpStore) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	if len(data) > hs.maxBlobSize {
		return "", stores.ErrMaxSizeExceeded
	}
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New(string(body))
	}
	key := hs.prefix + "/" + string(body)
	return key, nil
}

func (hs *HttpStore) MaxBlobSize() int {
	return hs.maxBlobSize
}

func (hs *HttpStore) Delete(ctx context.Context, key string) (err error) {
	key, err = hs.removePrefix(key)
	if err != nil {
		return err
	}
	u := hs.getURL(key)
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error not OK: %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}

func (hs *HttpStore) removePrefix(x string) (string, error) {
	if !strings.HasPrefix(x, hs.prefix) {
		return "", errors.New("Wrong key: " + x)
	}
	y := x[len(hs.prefix):]
	return y, nil
}

func (hs *HttpStore) getURL(x string) string {
	y := hs.endpoint
	if y[len(y)-1] != '/' {
		y += "/"
	}
	y += x
	return y
}
