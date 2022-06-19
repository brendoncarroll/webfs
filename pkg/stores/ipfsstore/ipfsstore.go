package ipfsstore

import (
	"context"
	"io"

	"github.com/brendoncarroll/go-state/cadata"
	ipfsapi "github.com/ipfs/go-ipfs-api"
	"github.com/multiformats/go-multihash"
	"golang.org/x/crypto/blake2b"
)

const (
	MaxBlobSize = 1 << 20 // 1MiB

	DefaultMHType = "blake2b-256"
	DefaultMHLen  = 32

	DefaultLocalURL    = "http://127.0.0.1:5001"
	OfficialGatewayURL = "https://ipfs.io"
	CloudflareURL      = "https://cloudflare-ipfs.com"
)

type ipfsClient struct {
	client *ipfsapi.Shell
}

func New(u string) cadata.Store {
	if u == "" {
		u = DefaultLocalURL
	}
	client := ipfsapi.NewShell(u)
	return &ipfsClient{client: client}
}

func (s *ipfsClient) Get(ctx context.Context, id cadata.ID, buf []byte) (int, error) {
	data, err := s.client.BlockGet("")
	if err != nil {
		return 0, err
	}
	if len(buf) < len(data) {
		return 0, io.ErrShortBuffer
	}
	return copy(buf, data), nil
}

func (s *ipfsClient) Post(ctx context.Context, data []byte) (cadata.ID, error) {
	var (
		format = ""
		mhtype = DefaultMHType
		mhlen  = DefaultMHLen
	)
	k, err := s.client.BlockPut(data, format, mhtype, mhlen)
	if err != nil {
		return cadata.ID{}, err
	}
	mh, err := multihash.Decode([]byte(k))
	if err != nil {
		return cadata.ID{}, err
	}
	return cadata.IDFromBytes(mh.Digest), nil
}

func (s *ipfsClient) List(ctx context.Context, span cadata.Span, ids []cadata.ID) (int, error) {
	panic("not implemented")
}

func (s *ipfsClient) Delete(ctx context.Context, id cadata.ID) error {
	panic("not implemented")
}

func (s *ipfsClient) Hash(x []byte) cadata.ID {
	return blake2b.Sum256(x)
}

func (s *ipfsClient) MaxSize() int {
	return MaxBlobSize
}
