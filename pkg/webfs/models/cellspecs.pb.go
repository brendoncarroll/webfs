// Code generated by protoc-gen-go. DO NOT EDIT.
// source: cellspecs.proto

package models

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import cryptocell "github.com/brendoncarroll/webfs/pkg/cells/cryptocell"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type CellSpec struct {
	// Types that are valid to be assigned to Spec:
	//	*CellSpec_Http
	//	*CellSpec_Crypto
	Spec                 isCellSpec_Spec `protobuf_oneof:"spec"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *CellSpec) Reset()         { *m = CellSpec{} }
func (m *CellSpec) String() string { return proto.CompactTextString(m) }
func (*CellSpec) ProtoMessage()    {}
func (*CellSpec) Descriptor() ([]byte, []int) {
	return fileDescriptor_cellspecs_e4c590859a1d9199, []int{0}
}
func (m *CellSpec) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CellSpec.Unmarshal(m, b)
}
func (m *CellSpec) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CellSpec.Marshal(b, m, deterministic)
}
func (dst *CellSpec) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CellSpec.Merge(dst, src)
}
func (m *CellSpec) XXX_Size() int {
	return xxx_messageInfo_CellSpec.Size(m)
}
func (m *CellSpec) XXX_DiscardUnknown() {
	xxx_messageInfo_CellSpec.DiscardUnknown(m)
}

var xxx_messageInfo_CellSpec proto.InternalMessageInfo

type isCellSpec_Spec interface {
	isCellSpec_Spec()
}

type CellSpec_Http struct {
	Http *HTTPCellSpec `protobuf:"bytes,1,opt,name=http,proto3,oneof"`
}

type CellSpec_Crypto struct {
	Crypto *CryptoCellSpec `protobuf:"bytes,128,opt,name=crypto,proto3,oneof"`
}

func (*CellSpec_Http) isCellSpec_Spec() {}

func (*CellSpec_Crypto) isCellSpec_Spec() {}

func (m *CellSpec) GetSpec() isCellSpec_Spec {
	if m != nil {
		return m.Spec
	}
	return nil
}

func (m *CellSpec) GetHttp() *HTTPCellSpec {
	if x, ok := m.GetSpec().(*CellSpec_Http); ok {
		return x.Http
	}
	return nil
}

func (m *CellSpec) GetCrypto() *CryptoCellSpec {
	if x, ok := m.GetSpec().(*CellSpec_Crypto); ok {
		return x.Crypto
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*CellSpec) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _CellSpec_OneofMarshaler, _CellSpec_OneofUnmarshaler, _CellSpec_OneofSizer, []interface{}{
		(*CellSpec_Http)(nil),
		(*CellSpec_Crypto)(nil),
	}
}

func _CellSpec_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*CellSpec)
	// spec
	switch x := m.Spec.(type) {
	case *CellSpec_Http:
		b.EncodeVarint(1<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Http); err != nil {
			return err
		}
	case *CellSpec_Crypto:
		b.EncodeVarint(128<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Crypto); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("CellSpec.Spec has unexpected type %T", x)
	}
	return nil
}

func _CellSpec_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*CellSpec)
	switch tag {
	case 1: // spec.http
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(HTTPCellSpec)
		err := b.DecodeMessage(msg)
		m.Spec = &CellSpec_Http{msg}
		return true, err
	case 128: // spec.crypto
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(CryptoCellSpec)
		err := b.DecodeMessage(msg)
		m.Spec = &CellSpec_Crypto{msg}
		return true, err
	default:
		return false, nil
	}
}

func _CellSpec_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*CellSpec)
	// spec
	switch x := m.Spec.(type) {
	case *CellSpec_Http:
		s := proto.Size(x.Http)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case *CellSpec_Crypto:
		s := proto.Size(x.Crypto)
		n += 2 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

type HTTPCellSpec struct {
	Url                  string   `protobuf:"bytes,1,opt,name=url,proto3" json:"url,omitempty"`
	AuthHeader           string   `protobuf:"bytes,2,opt,name=auth_header,json=authHeader,proto3" json:"auth_header,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *HTTPCellSpec) Reset()         { *m = HTTPCellSpec{} }
func (m *HTTPCellSpec) String() string { return proto.CompactTextString(m) }
func (*HTTPCellSpec) ProtoMessage()    {}
func (*HTTPCellSpec) Descriptor() ([]byte, []int) {
	return fileDescriptor_cellspecs_e4c590859a1d9199, []int{1}
}
func (m *HTTPCellSpec) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_HTTPCellSpec.Unmarshal(m, b)
}
func (m *HTTPCellSpec) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_HTTPCellSpec.Marshal(b, m, deterministic)
}
func (dst *HTTPCellSpec) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HTTPCellSpec.Merge(dst, src)
}
func (m *HTTPCellSpec) XXX_Size() int {
	return xxx_messageInfo_HTTPCellSpec.Size(m)
}
func (m *HTTPCellSpec) XXX_DiscardUnknown() {
	xxx_messageInfo_HTTPCellSpec.DiscardUnknown(m)
}

var xxx_messageInfo_HTTPCellSpec proto.InternalMessageInfo

func (m *HTTPCellSpec) GetUrl() string {
	if m != nil {
		return m.Url
	}
	return ""
}

func (m *HTTPCellSpec) GetAuthHeader() string {
	if m != nil {
		return m.AuthHeader
	}
	return ""
}

type CryptoCellSpec struct {
	Inner                *CellSpec          `protobuf:"bytes,1,opt,name=inner,proto3" json:"inner,omitempty"`
	PrivateEntity        *cryptocell.Entity `protobuf:"bytes,2,opt,name=private_entity,json=privateEntity,proto3" json:"private_entity,omitempty"`
	PublicEntity         *cryptocell.Entity `protobuf:"bytes,3,opt,name=public_entity,json=publicEntity,proto3" json:"public_entity,omitempty"`
	Who                  *cryptocell.Who    `protobuf:"bytes,4,opt,name=who,proto3" json:"who,omitempty"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *CryptoCellSpec) Reset()         { *m = CryptoCellSpec{} }
func (m *CryptoCellSpec) String() string { return proto.CompactTextString(m) }
func (*CryptoCellSpec) ProtoMessage()    {}
func (*CryptoCellSpec) Descriptor() ([]byte, []int) {
	return fileDescriptor_cellspecs_e4c590859a1d9199, []int{2}
}
func (m *CryptoCellSpec) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CryptoCellSpec.Unmarshal(m, b)
}
func (m *CryptoCellSpec) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CryptoCellSpec.Marshal(b, m, deterministic)
}
func (dst *CryptoCellSpec) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CryptoCellSpec.Merge(dst, src)
}
func (m *CryptoCellSpec) XXX_Size() int {
	return xxx_messageInfo_CryptoCellSpec.Size(m)
}
func (m *CryptoCellSpec) XXX_DiscardUnknown() {
	xxx_messageInfo_CryptoCellSpec.DiscardUnknown(m)
}

var xxx_messageInfo_CryptoCellSpec proto.InternalMessageInfo

func (m *CryptoCellSpec) GetInner() *CellSpec {
	if m != nil {
		return m.Inner
	}
	return nil
}

func (m *CryptoCellSpec) GetPrivateEntity() *cryptocell.Entity {
	if m != nil {
		return m.PrivateEntity
	}
	return nil
}

func (m *CryptoCellSpec) GetPublicEntity() *cryptocell.Entity {
	if m != nil {
		return m.PublicEntity
	}
	return nil
}

func (m *CryptoCellSpec) GetWho() *cryptocell.Who {
	if m != nil {
		return m.Who
	}
	return nil
}

func init() {
	proto.RegisterType((*CellSpec)(nil), "webfs.CellSpec")
	proto.RegisterType((*HTTPCellSpec)(nil), "webfs.HTTPCellSpec")
	proto.RegisterType((*CryptoCellSpec)(nil), "webfs.CryptoCellSpec")
}

func init() { proto.RegisterFile("cellspecs.proto", fileDescriptor_cellspecs_e4c590859a1d9199) }

var fileDescriptor_cellspecs_e4c590859a1d9199 = []byte{
	// 285 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x91, 0xc1, 0x6a, 0xb3, 0x40,
	0x10, 0xc7, 0x3f, 0x3f, 0x8d, 0x98, 0xd1, 0x24, 0x65, 0x4b, 0x41, 0xbc, 0xb4, 0x08, 0x85, 0xf6,
	0x22, 0xc1, 0x3e, 0x41, 0x13, 0x0a, 0x1e, 0x8b, 0x0d, 0x14, 0x7a, 0x09, 0xba, 0xd9, 0xa2, 0xb0,
	0x75, 0x97, 0x75, 0x6d, 0xc8, 0xad, 0xaf, 0xd5, 0xb7, 0x2b, 0xce, 0xae, 0x90, 0xd2, 0xdb, 0xf8,
	0x9b, 0xdf, 0x7f, 0x66, 0x70, 0x61, 0x45, 0x19, 0xe7, 0xbd, 0x64, 0xb4, 0xcf, 0xa4, 0x12, 0x5a,
	0x90, 0xd9, 0x91, 0xd5, 0xef, 0x7d, 0x12, 0x51, 0x75, 0x92, 0x5a, 0x18, 0x98, 0xcc, 0x29, 0x95,
	0xa6, 0x4c, 0x05, 0x04, 0x5b, 0xc6, 0xf9, 0x8b, 0x64, 0x94, 0xdc, 0x83, 0xd7, 0x68, 0x2d, 0x63,
	0xe7, 0xc6, 0xb9, 0x0b, 0xf3, 0xcb, 0x0c, 0xa3, 0x59, 0xb1, 0xdb, 0x3d, 0x4f, 0x4a, 0xf1, 0xaf,
	0x44, 0x85, 0xac, 0xc1, 0x37, 0x13, 0xe3, 0x2f, 0x63, 0x5f, 0x59, 0x7b, 0x8b, 0xf4, 0xcc, 0xb7,
	0xde, 0xc6, 0x07, 0x6f, 0xbc, 0x2b, 0x7d, 0x84, 0xe8, 0x7c, 0x22, 0xb9, 0x00, 0x77, 0x50, 0x1c,
	0x77, 0xce, 0xcb, 0xb1, 0x24, 0xd7, 0x10, 0x56, 0x83, 0x6e, 0xf6, 0x0d, 0xab, 0x0e, 0x4c, 0xc5,
	0xff, 0xb1, 0x03, 0x23, 0x2a, 0x90, 0xa4, 0xdf, 0x0e, 0x2c, 0x7f, 0xef, 0x21, 0xb7, 0x30, 0x6b,
	0xbb, 0x8e, 0x29, 0x7b, 0xfb, 0x6a, 0xba, 0xc6, 0xf6, 0x4b, 0xd3, 0x25, 0x39, 0x2c, 0xa5, 0x6a,
	0x3f, 0x2b, 0xcd, 0xf6, 0xac, 0xd3, 0xad, 0x3e, 0xe1, 0xf4, 0x30, 0x0f, 0xb3, 0xf1, 0x8f, 0x3c,
	0x21, 0x2a, 0x17, 0x56, 0x31, 0x9f, 0x64, 0x0d, 0x0b, 0x39, 0xd4, 0xbc, 0xa5, 0x53, 0xc4, 0xfd,
	0x1b, 0x89, 0x8c, 0x61, 0x13, 0x09, 0xb8, 0xc7, 0x46, 0xc4, 0x1e, 0x7a, 0x01, 0x7a, 0xaf, 0x8d,
	0x28, 0x47, 0xb8, 0x09, 0xde, 0xfc, 0x0f, 0x71, 0x60, 0xbc, 0xaf, 0x7d, 0x7c, 0x80, 0x87, 0x9f,
	0x00, 0x00, 0x00, 0xff, 0xff, 0xb9, 0xa1, 0x13, 0xfc, 0xb3, 0x01, 0x00, 0x00,
}
