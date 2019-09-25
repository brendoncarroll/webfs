package webref

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/brendoncarroll/webfs/pkg/stores"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

const (
	CodecJSON     = "JSON"
	CodecProtobuf = "PB"
)

func Load(ctx context.Context, s stores.Read, ref Ref, x interface{}) error {
	data, err := Get(ctx, s, ref)
	if err != nil {
		return err
	}
	codec := ""
	if ref.Attrs != nil {
		codec = ref.Attrs["codec"]
	}
	return Decode(codec, data, x)
}

func Store(ctx context.Context, s stores.Post, opts Options, x interface{}) (*Ref, error) {
	codec := opts.Attrs["codec"]
	if codec == "" {
		codec = CodecJSON
	}
	data, err := Encode(codec, x)
	if err != nil {
		return nil, err
	}

	ref, err := Post(ctx, s, opts, data)
	if err != nil {
		return nil, err
	}
	if ref.Attrs == nil {
		ref.Attrs = map[string]string{}
	}
	ref.Attrs["codec"] = codec
	return ref, nil
}

func SizeOf(s stores.Post, o Options, x interface{}) int {
	codec := o.Attrs["codec"]
	switch codec {
	case CodecProtobuf:
		pm, ok := x.(proto.Message)
		if !ok {
			panic("can't use protobuf encoding with non proto.Message")
		}
		return proto.Size(pm)
	default:
		data, err := Encode(codec, x)
		if err != nil {
			panic(err)
		}
		return len(data)
	}
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
			log.Printf("WARN: using regular json encoder for type %T\n", x)
			data, err = json.Marshal(x)
			if err != nil {
				return nil, err
			}
		}
	case CodecProtobuf:
		if pm, ok := x.(proto.Message); ok {
			data, err = proto.Marshal(pm)
		} else {
			return nil, errors.New("cannot marshal non-protobuf as protobuf")
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
			log.Printf("WARN: using regular json decoder for type %T\n", x)
			if err := json.Unmarshal(data, x); err != nil {
				return err
			}
		}

	case CodecProtobuf:
		if pm, ok := x.(proto.Message); ok {
			return proto.Unmarshal(data, pm)
		} else {
			return errors.New("cannot unmarshal into non-protobuf")
		}

	default:
		return errors.New("unrecognized codec: " + codec)
	}
	return nil
}
