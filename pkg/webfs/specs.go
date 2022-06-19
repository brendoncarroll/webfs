package webfs

import (
	"crypto/cipher"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	bcclient "github.com/blobcache/blobcache/client/go_client"
	"github.com/blobcache/blobcache/pkg/blobcache"
	"github.com/brendoncarroll/go-state/cadata"
	"github.com/brendoncarroll/go-state/cadata/fsstore"
	"github.com/brendoncarroll/go-state/cells"
	"github.com/brendoncarroll/go-state/cells/cryptocell"
	"github.com/brendoncarroll/go-state/cells/httpcell"
	"github.com/brendoncarroll/go-state/posixfs"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/brendoncarroll/webfs/pkg/cells/filecell"
	"github.com/brendoncarroll/webfs/pkg/cells/gotcells"
	"github.com/brendoncarroll/webfs/pkg/stores/ipfsstore"
)

// VolumeSpec is a specification for a Volume.
type VolumeSpec struct {
	Cell  CellSpec  `json:"cell"`
	Store StoreSpec `json:"store"`
	Salt  []byte    `json:"salt"`
}

func (vs VolumeSpec) Fingerprint() [32]byte {
	data, _ := json.Marshal(vs)
	return Hash(data)
}

// ParseVolumeSpec parses a JSON formatted VolumeSpec from x.
func ParseVolumeSpec(x []byte) (*VolumeSpec, error) {
	var spec VolumeSpec
	if err := json.Unmarshal(x, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func MarshalVolumeSpec(x VolumeSpec) ([]byte, error) {
	return json.MarshalIndent(x, "", "  ")
}

// CellSpec is a specification for a Cell
type CellSpec struct {
	Memory  *struct{}       `json:"memory,omitempty"`
	File    *string         `json:"file,omitempty"`
	HTTP    *HTTPCellSpec   `json:"http,omitempty"`
	Literal json.RawMessage `json:"literal,omitempty"`

	AEAD      *AEADCellSpec      `json:"aead,omitempty"`
	GotBranch *GotBranchCellSpec `json:"got_branch,omitempty"`
}

type HTTPCellSpec struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

type AEADCellSpec struct {
	Inner  CellSpec `json:"inner"`
	Algo   string   `json:"algo"`
	Secret []byte   `json:"secret"`
}

type GotBranchCellSpec struct {
	Inner   CellSpec  `json:"inner"`
	VCStore StoreSpec `json:"vc_store"`
}

type StoreSpec struct {
	Memory    *struct{}           `json:"memory,omitempty"`
	FS        *string             `json:"fs,omitempty"`
	HTTP      HTTPStoreSpec       `json:"http,omitempty"`
	Blobcache *BlobcacheStoreSpec `json:"blobcache,omitempty"`
	IPFS      *IPFSStoreSpec      `json:"ipfs,omitempty"`
}

type HTTPStoreSpec struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type BlobcacheStoreSpec struct{}

type IPFSStoreSpec struct{}

func (fs *FS) makeVolume(spec VolumeSpec) (*Volume, error) {
	cell, err := fs.makeCell(spec.Cell)
	if err != nil {
		return nil, err
	}
	store, err := fs.makeStore(spec.Store)
	if err != nil {
		return nil, err
	}
	return &Volume{
		Cell:  cell,
		Store: store,
	}, nil
}

func (fs *FS) makeCell(spec CellSpec) (cells.Cell, error) {
	switch {
	case spec.Memory != nil:
		return cells.NewMem(1 << 16), nil
	case spec.File != nil:
		return filecell.New(fs.fs, *spec.File), nil
	case spec.HTTP != nil:
		return httpcell.New(httpcell.Spec{
			URL:     spec.HTTP.URL,
			Headers: spec.HTTP.Headers,
		}), nil
	case spec.Literal != nil:
		panic("not implemented")

	case spec.AEAD != nil:
		inner, err := fs.makeCell(spec.AEAD.Inner)
		if err != nil {
			return nil, err
		}
		var aead cipher.AEAD
		algo := strings.ToLower(spec.AEAD.Algo)
		switch algo {
		case "chacha20poly1305":
			aead, err = chacha20poly1305.NewX(spec.AEAD.Secret)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported AEAD: %q", algo)
		}
		return cryptocell.NewAEAD(inner, aead), nil
	case spec.GotBranch != nil:
		inner, err := fs.makeCell(spec.GotBranch.Inner)
		if err != nil {
			return nil, err
		}
		// vcstore, err := fs.makeStore(spec.GotBranch.VCStore)
		// if err != nil {
		// 	return nil, err
		// }
		return gotcells.NewBranch(inner, nil, nil), nil
	default:
		return nil, errors.New("empty cell spec")
	}
}

func (fs *FS) makeStore(spec StoreSpec) (cadata.Store, error) {
	switch {
	case spec.Memory != nil:
		return cadata.NewMem(Hash, MaxBlobSize), nil
	case spec.FS != nil:
		if err := os.MkdirAll(*spec.FS, 0o755); err != nil {
			return nil, err
		}
		pfs := posixfs.NewDirFS(*spec.FS)
		return fsstore.New(pfs, Hash, MaxBlobSize), nil
	case spec.Blobcache != nil:
		c, err := bcclient.NewClient(fs.config.blobcacheEndpoint)
		if err != nil {
			return nil, err
		}
		// TODO: need to set handle
		return blobcache.NewStore(c, blobcache.Handle{}), nil
	case spec.IPFS != nil:
		return ipfsstore.New(fs.config.ipfs), nil
	default:
		return nil, errors.New("empty store spec")
	}
}
