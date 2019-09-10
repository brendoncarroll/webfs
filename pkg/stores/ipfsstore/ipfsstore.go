package ipfsstore

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/stores"
	ipfsapi "github.com/ipfs/go-ipfs-api"
)

const (
	MaxBlobSize = 1 << 20 // 1MiB

	DefaultMHType = "sha2-256"
	DefaultMHLen  = 32

	DefaultLocalURL    = "http://127.0.0.1:5001"
	OfficialGatewayURL = "https://ipfs.io"
	CloudflareURL      = "https://cloudflare-ipfs.com"

	urlPrefix = "ipfs://"
)

type IPFSStore = stores.ReadWriteOnce

func New(u string) (IPFSStore, error) {
	if u == "" {
		u = DefaultLocalURL
	}
	client := ipfsapi.NewShell(u)
	if client.IsUp() {
		return &ipfsClient{
			client: client,
		}, nil
	}

	log.Printf("ipfs-node: %s is down\n", u)
	// no local node try to setup a gateway
	urls := []string{
		CloudflareURL,
		OfficialGatewayURL,
	}
	for _, u := range urls {
		gateway := &ipfsGateway{endpoint: u}
		if gateway.IsUp() {
			log.Printf("ipfs-gateway %s is up\n", u)
			return gateway, nil
		}
	}

	return nil, fmt.Errorf("no available ipfs gateway")
}

type ipfsClient struct {
	client *ipfsapi.Shell
}

func (s *ipfsClient) Get(ctx context.Context, key string) ([]byte, error) {
	if !strings.HasPrefix(key, urlPrefix) {
		return nil, errors.New("Invalid key: " + key)
	}
	p := key[len(urlPrefix):]
	return s.client.BlockGet(p)
}

func (s *ipfsClient) Post(ctx context.Context, key string, data []byte) (string, error) {
	var (
		format = ""
		mhtype = DefaultMHType
		mhlen  = DefaultMHLen
	)
	k, err := s.client.BlockPut(data, format, mhtype, mhlen)
	if err != nil {
		return "", err
	}
	return urlPrefix + k, nil
}

func (s *ipfsClient) MaxBlobSize() int {
	return MaxBlobSize
}

type ipfsGateway struct {
	endpoint string
}

func (s *ipfsGateway) IsUp() bool {
	c := http.DefaultClient
	res, err := c.Head(s.endpoint)
	if err != nil {
		return false
	}
	return res.StatusCode == http.StatusOK
}

func (s *ipfsGateway) Get(ctx context.Context, key string) ([]byte, error) {
	if !strings.HasPrefix(key, urlPrefix) {
		return nil, errors.New("Invalid key: " + key)
	}
	key = key[len(urlPrefix):]

	p := fmt.Sprintf("%s/ipfs/%s", s.endpoint, key)
	res, err := http.Get(p)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}

func (s *ipfsGateway) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	return "", fmt.Errorf("cannot post to IPFS gateway")
}

func (s *ipfsGateway) MaxBlobSize() int {
	return 0
}
