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
	s := &HttpStore{
		endpoint: endpoint,
		prefix:   prefix,
		hc:       http.DefaultClient,
		headers:  headers,
	}

	mbsURL := endpoint + "/.maxBlobSize"
	r := s.newRequest(context.TODO(), http.MethodGet, mbsURL, nil)
	resp, err := s.hc.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errorFromRes(resp)
	}

	if _, err := fmt.Fscanf(resp.Body, "%d", &s.maxBlobSize); err != nil {
		return nil, err
	}
	return s, nil
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
		return nil, errorFromRes(resp)
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
		return "", errorFromRes(resp)
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
		return errorFromRes(resp)
	}
	return nil
}

func (hs *HttpStore) removePrefix(x string) (string, error) {
	if !strings.HasPrefix(x, hs.prefix) {
		return "", errors.New("Wrong key: " + x)
	}
	y := x[len(hs.prefix):]
	if y[0] == '/' {
		return y[1:], nil
	}
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

func errorFromRes(r *http.Response) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
	}
	msg := string(body)
	return fmt.Errorf("%s: %s", r.Status, msg)
}
