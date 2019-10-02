package httpstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/multiformats/go-multihash"
)

type HttpStore struct {
	endpoint    string
	prefix      string
	maxBlobSize int
	headers     map[string]string
	hc          *http.Client
}

func New(endpoint string, prefix string, headers map[string]string) (*HttpStore, error) {
	mbsUrl := endpoint + "/.maxBlobSize"
	resp, err := http.Get(mbsUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var maxBlobSize int
	if _, err := fmt.Fscanf(resp.Body, "%d", &maxBlobSize); err != nil {
		return nil, err
	}

	return &HttpStore{
		endpoint:    endpoint,
		maxBlobSize: maxBlobSize,
		prefix:      prefix,
		hc:          http.DefaultClient,
		headers:     headers,
	}, nil
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

	req := hs.newRequest(ctx, http.MethodGet, hs.getURL(key2), nil)

	resp, err := hs.hc.Do(req)
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
	req := hs.newRequest(ctx, http.MethodHead, u, nil)
	resp, err := hs.hc.Do(req)
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
	req := hs.newRequest(ctx, http.MethodPost, hs.endpoint, buf)
	resp, err := hs.hc.Do(req)
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
		return "", fmt.Errorf("endpoint=%s status=%d body=%s", hs.endpoint, resp.StatusCode, string(body))
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

	req := hs.newRequest(ctx, http.MethodDelete, u, nil)

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

func (hs *HttpStore) newRequest(ctx context.Context, method, u string, body io.Reader) *http.Request {
	r, err := http.NewRequest(method, u, body)
	if err != nil {
		panic(err)
	}
	r = r.WithContext(ctx)
	for k, v := range hs.headers {
		r.Header.Set(k, v)
	}
	return r
}
