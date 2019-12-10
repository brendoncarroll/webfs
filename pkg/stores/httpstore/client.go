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

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/multiformats/go-multihash"
)

type HttpStore struct {
	endpoint    string
	maxBlobSize int
	headers     map[string]string
	hc          *http.Client
	validateMH  bool
}

func New(endpoint string, headers map[string]string) *HttpStore {
	s := &HttpStore{
		endpoint:    endpoint,
		hc:          http.DefaultClient,
		headers:     headers,
		maxBlobSize: -1,
		validateMH:  true,
	}
	return s
}

func (hs *HttpStore) Init(ctx context.Context) error {
	mbsURL := hs.endpoint + "/.maxBlobSize"
	r := hs.newRequest(ctx, http.MethodGet, mbsURL, nil)
	resp, err := hs.hc.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errorFromRes(resp)
	}

	if _, err := fmt.Fscanf(resp.Body, "%d", &hs.maxBlobSize); err != nil {
		return err
	}

	return nil
}

func (hs *HttpStore) Get(ctx context.Context, key string) ([]byte, error) {
	req := hs.newRequest(ctx, http.MethodGet, hs.getURL(key), nil)

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

	if hs.validateMH {
		mhBytes, err := base64.URLEncoding.DecodeString(key)
		if err != nil {
			return nil, err
		}
		want, err := multihash.Decode(mhBytes)
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
	}

	return data, nil
}

func (hs *HttpStore) Check(ctx context.Context, key string) (err error) {
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
	if len(prefix) > 0 {
		return "", errors.New("prefix must be empty")
	}
	if len(data) > hs.maxBlobSize {
		return "", stores.ErrMaxSizeExceeded
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

	if hs.validateMH {
		mhBytes := make([]byte, enc.DecodedLen(len(body)))
		n, err := enc.Decode(mhBytes, body)
		if err != nil {
			return "", err
		}
		mhBytes = mhBytes[:n]

		mh, err := multihash.Decode(mhBytes)
		if err != nil {
			return "", err
		}
		mh2, err := multihash.Sum(data, mh.Code, mh.Length)
		if err != nil {
			return "", err
		}
		if bytes.Compare(mhBytes, mh2) != 0 {
			return "", errors.New("server gave bad multihash for data")
		}
	}

	return string(body), nil
}

func (hs *HttpStore) MaxBlobSize() int {
	return hs.maxBlobSize
}

func (hs *HttpStore) Delete(ctx context.Context, key string) (err error) {
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
