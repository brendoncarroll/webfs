package ipfsstore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/brendoncarroll/webfs/pkg/stores"
	ipfsapi "github.com/ipfs/go-ipfs-api"
)

const (
	MaxBlobSize = 1 << 20 // 1MiB

	DefaultMHType = "sha2-256"
	DefaultMHLen  = 32

	OfficialGatewayURL = "https://ipfs.io/"
	DefaultLocalURL    = "http://127.0.0.1:5001"

	urlPrefix = "ipfs://"
)

type IPFSStore = stores.ReadWriteOnce

func New(url string) (IPFSStore, error) {
	if url == "" {
		url = DefaultLocalURL
	}
	urls := []string{
		url,
		OfficialGatewayURL,
	}
	for _, u := range urls {
		client := ipfsapi.NewShell(url)
		if !client.IsUp() {
			log.Printf("ipfsstore: %s is down\n", u)
		}
		return &ipfsGateway{
			client: client,
		}, nil
	}
	return nil, fmt.Errorf("no available ipfs gateway")
}

type ipfsGateway struct {
	isLocal bool
	client  *ipfsapi.Shell
}

func (s *ipfsGateway) Get(ctx context.Context, key string) ([]byte, error) {
	if !strings.HasPrefix(key, urlPrefix) {
		return nil, errors.New("Invalid key: " + key)
	}
	p := key[len(urlPrefix):]
	return s.client.BlockGet(p)
}

func (s *ipfsGateway) Post(ctx context.Context, key string, data []byte) (string, error) {
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

func (s *ipfsGateway) MaxBlobSize() int {
	return MaxBlobSize
}
