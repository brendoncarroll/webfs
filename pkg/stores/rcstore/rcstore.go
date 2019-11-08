package rcstore

import (
	"context"
	"io/ioutil"
	"log"

	rcfs "github.com/rclone/rclone/fs"
)

const MaxBlobSize = 1 << 20

type Spec struct {
	FS rcfs.Fs
}

type RCloneStore struct {
	fs rcfs.Fs
}

func New(spec Spec) *RCloneStore {
	return &RCloneStore{fs: spec.FS}
}

func (s *RCloneStore) Get(ctx context.Context, key string) ([]byte, error) {
	remote := key
	o, err := s.fs.NewObject(ctx, remote)
	if err != nil {
		return nil, err
	}
	rc, err := o.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rc.Close(); err != nil {
			log.Println(err)
		}
	}()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *RCloneStore) Post(ctx context.Context, prefix string, data []byte) (string, error) {
	panic("not implemented")
}

func (s *RCloneStore) MaxBlobSize() int {
	return MaxBlobSize
}
