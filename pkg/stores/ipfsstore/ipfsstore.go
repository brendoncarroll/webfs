package ipfsstore

import (
	"context"
	"errors"
	"strings"

	ipfsapi "github.com/ipfs/go-ipfs-api"
)

const MaxBlobSize = 1 << 20 // 1MiB

type IPFSStore struct {
	client *ipfsapi.Shell
}

func New(url string) *IPFSStore {
	return &IPFSStore{
		client: ipfsapi.NewShell(url),
	}
}

func (s *IPFSStore) Get(ctx context.Context, key string) ([]byte, error) {
	const urlPrefix = "ipfs://"
	if !strings.HasPrefix(key, urlPrefix) {
		return nil, errors.New("Invalid key: " + key)
	}
	p := "ipfs/" + key[len(urlPrefix):]
	return s.client.BlockGet(p)
}

func (s *IPFSStore) Post(ctx context.Context, key string, data []byte) (string, error) {
	var (
		format = ""
		mhtype = ""
		mhlen  = 0
	)
	k, err := s.client.BlockPut(data, format, mhtype, mhlen)
	if err != nil {
		return "", err
	}
	return k, nil
}

func (s *IPFSStore) MaxBlobSize() int {
	return MaxBlobSize
}
