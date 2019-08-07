package webref

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

const (
	CodecJSON     = "JSON"
	CodecProtobuf = "PB"
)

func Load(ctx context.Context, s Read, ref Ref, x interface{}) error {
	data, err := s.Get(ctx, ref)
	if err != nil {
		return err
	}
	codec := ""
	if ref.Attrs != nil {
		codec = ref.Attrs["codec"]
	}
	return Decode(codec, data, x)
}

func Store(ctx context.Context, s WriteOnce, x interface{}) (*Ref, error) {
	o := s.Options()

	codec := o.Attrs["codec"]
	if codec == "" {
		codec = CodecJSON
	}
	data, err := Encode(codec, x)
	if err != nil {
		return nil, err
	}
	ref, err := s.Post(ctx, data)
	if err != nil {
		return nil, err
	}
	if ref.Attrs == nil {
		ref.Attrs = map[string]string{}
	}
	ref.Attrs["codec"] = codec
	return ref, nil
}

func SizeOf(s WriteOnce, x interface{}) int {
	o := s.Options()
	codec := o.Attrs["codec"]
	data, err := Encode(codec, x)
	if err != nil {
		panic(err)
	}
	return len(data)
}

func Encode(codec string, x interface{}) (data []byte, err error) {
	switch codec {
	case CodecJSON:
		pm, ok := x.(proto.Message)
		if ok {
			mer := jsonpb.Marshaler{
				EnumsAsInts: false,
			}
			buf := bytes.Buffer{}
			err = mer.Marshal(&buf, pm)
			if err != nil {
				return nil, err
			}
			data = buf.Bytes()
		} else {
			data, err = json.Marshal(x)
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, errors.New("unrecognized codec: " + codec)
	}
	return data, err
}

func Decode(codec string, data []byte, x interface{}) error {
	switch codec {
	case CodecJSON:
		pm, ok := x.(proto.Message)
		if ok {
			if err := jsonpb.Unmarshal(bytes.NewBuffer(data), pm); err != nil {
				return err
			}
		} else {
			if err := json.Unmarshal(data, x); err != nil {
				return err
			}
		}

	default:
		return errors.New("unrecognized codec: " + codec)
	}
	return nil
}
