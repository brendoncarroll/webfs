package webref

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

const (
	CodecJSON     = "JSON"
	CodecProtobuf = "PB"
)

func GetAndDecode(ctx context.Context, s Getter, ref Ref, x interface{}) error {
	aref, ok := ref.Ref.(*Ref_Annotated)
	if !ok {
		return errors.New("can't load from non-annotated ref")
	}
	annotations := aref.Annotated.Annotations
	codec := ""
	if annotations != nil {
		codec = annotations["codec"]
	}

	data, err := s.Get(ctx, &ref)
	if err != nil {
		return err
	}

	return Decode(codec, data, x)
}

func EncodeAndPost(ctx context.Context, s Poster, x interface{}) (*Ref, error) {
	codec := GetCodecCtx(ctx)
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
	aref := &Annotated{
		Ref: ref,
		Annotations: map[string]string{
			"codec": codec,
		},
	}
	return &Ref{
		Ref: &Ref_Annotated{aref},
	}, nil
}

func SizeOf(codec string, x interface{}) int {
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
		log.Println("CODEC", codec)
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

func (r *Ref) MarshalJSON() ([]byte, error) {
	m := jsonpb.Marshaler{}
	buf := &bytes.Buffer{}
	if err := m.Marshal(buf, r); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (r *Ref) UnmarshalJSON(data []byte) error {
	return jsonpb.Unmarshal(bytes.NewBuffer(data), r)
}
