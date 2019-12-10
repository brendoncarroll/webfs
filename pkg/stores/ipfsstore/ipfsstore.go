package ipfsstore

import (
	"context"
	"errors"

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
)

type IPFSStore = stores.ReadPost

func New(u string) IPFSStore {
	if u == "" {
		u = DefaultLocalURL
	}
	client := ipfsapi.NewShell(u)
	return &ipfsClient{client: client}
}

type ipfsClient struct {
	client *ipfsapi.Shell
}

func (s *ipfsClient) Get(ctx context.Context, key string) ([]byte, error) {
	return s.client.BlockGet(key)
}

func (s *ipfsClient) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	if len(prefix) > 0 {
		return "", errors.New("prefix must be empty")
	}
	var (
		format = ""
		mhtype = DefaultMHType
		mhlen  = DefaultMHLen
	)
	k, err := s.client.BlockPut(data, format, mhtype, mhlen)
	if err != nil {
		return "", err
	}
	return k, nil
}

func (s *ipfsClient) MaxBlobSize() int {
	return MaxBlobSize
}
