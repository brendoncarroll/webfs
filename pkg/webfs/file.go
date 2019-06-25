package webfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"

	"github.com/brendoncarroll/webfs/pkg/merkleds"
	"golang.org/x/crypto/sha3"
)

type File struct {
	Checksum []byte `json:"checksum"`
	Size     uint64 `json:"size"`

	Tree *merkleds.Tree `json:"tree"`
}

func NewFile(ctx context.Context, s ReadWriteOnce, r io.Reader) (*File, error) {
	if r == nil {
		r = &bytes.Buffer{}
	}
	f := &File{Tree: merkleds.NewTree()}
	h := sha3.New256()

	buf := make([]byte, s.MaxBlobSize())
	if len(buf) == 0 {
		panic("max blob size 0")
	}
	for {
		n, err := r.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		data := buf[:n]

		n, err = h.Write(data)
		if err != nil {
			return nil, err
		}

		ref, err := s.Post(ctx, data)
		if err != nil {
			return nil, err
		}

		key := [8]byte{}
		binary.BigEndian.PutUint64(key[:], f.Size)
		f.Tree, err = f.Tree.Put(ctx, s, key[:], *ref)
		if err != nil {
			return nil, err
		}

		f.Size += uint64(n)
	}
	f.Checksum = h.Sum(f.Checksum)

	return f, nil
}

func (f *File) Reader(store Read) io.ReadCloser {
	d, _ := json.Marshal(f.Tree.Entries)
	log.Println(string(d))
	return &FileReader{
		tree:  f.Tree,
		store: store,
		ref:   f.Tree.Entries[0].Ref,
	}
}

type FileReader struct {
	offset uint64
	buf    bytes.Buffer
	ref    Ref
	store  Read
	tree   *merkleds.Tree
}

func (fr *FileReader) Read(p []byte) (n int, err error) {
	ctx := context.TODO()
	data, err := fr.store.Get(ctx, fr.ref)
	if err != nil {
		return 0, err
	}
	n = copy(p, data)
	return n, io.EOF
}

func (fr *FileReader) Close() error {
	return nil
}
