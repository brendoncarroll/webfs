// Code generated by protoc-gen-go. DO NOT EDIT.
// source: storespecs.proto

package models

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type HTTPStoreSpec struct {
	Prefix               string   `protobuf:"bytes,1,opt,name=prefix,proto3" json:"prefix,omitempty"`
	Endpoint             string   `protobuf:"bytes,2,opt,name=endpoint,proto3" json:"endpoint,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *HTTPStoreSpec) Reset()         { *m = HTTPStoreSpec{} }
func (m *HTTPStoreSpec) String() string { return proto.CompactTextString(m) }
func (*HTTPStoreSpec) ProtoMessage()    {}
func (*HTTPStoreSpec) Descriptor() ([]byte, []int) {
	return fileDescriptor_storespecs_16cb4fd0cf739dc8, []int{0}
}
func (m *HTTPStoreSpec) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_HTTPStoreSpec.Unmarshal(m, b)
}
func (m *HTTPStoreSpec) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_HTTPStoreSpec.Marshal(b, m, deterministic)
}
func (dst *HTTPStoreSpec) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HTTPStoreSpec.Merge(dst, src)
}
func (m *HTTPStoreSpec) XXX_Size() int {
	return xxx_messageInfo_HTTPStoreSpec.Size(m)
}
func (m *HTTPStoreSpec) XXX_DiscardUnknown() {
	xxx_messageInfo_HTTPStoreSpec.DiscardUnknown(m)
}

var xxx_messageInfo_HTTPStoreSpec proto.InternalMessageInfo

func (m *HTTPStoreSpec) GetPrefix() string {
	if m != nil {
		return m.Prefix
	}
	return ""
}

func (m *HTTPStoreSpec) GetEndpoint() string {
	if m != nil {
		return m.Endpoint
	}
	return ""
}

type IPFSStoreSpec struct {
	Endpoint             string   `protobuf:"bytes,1,opt,name=endpoint,proto3" json:"endpoint,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *IPFSStoreSpec) Reset()         { *m = IPFSStoreSpec{} }
func (m *IPFSStoreSpec) String() string { return proto.CompactTextString(m) }
func (*IPFSStoreSpec) ProtoMessage()    {}
func (*IPFSStoreSpec) Descriptor() ([]byte, []int) {
	return fileDescriptor_storespecs_16cb4fd0cf739dc8, []int{1}
}
func (m *IPFSStoreSpec) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_IPFSStoreSpec.Unmarshal(m, b)
}
func (m *IPFSStoreSpec) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_IPFSStoreSpec.Marshal(b, m, deterministic)
}
func (dst *IPFSStoreSpec) XXX_Merge(src proto.Message) {
	xxx_messageInfo_IPFSStoreSpec.Merge(dst, src)
}
func (m *IPFSStoreSpec) XXX_Size() int {
	return xxx_messageInfo_IPFSStoreSpec.Size(m)
}
func (m *IPFSStoreSpec) XXX_DiscardUnknown() {
	xxx_messageInfo_IPFSStoreSpec.DiscardUnknown(m)
}

var xxx_messageInfo_IPFSStoreSpec proto.InternalMessageInfo

func (m *IPFSStoreSpec) GetEndpoint() string {
	if m != nil {
		return m.Endpoint
	}
	return ""
}

func init() {
	proto.RegisterType((*HTTPStoreSpec)(nil), "webfs.HTTPStoreSpec")
	proto.RegisterType((*IPFSStoreSpec)(nil), "webfs.IPFSStoreSpec")
}

func init() { proto.RegisterFile("storespecs.proto", fileDescriptor_storespecs_16cb4fd0cf739dc8) }

var fileDescriptor_storespecs_16cb4fd0cf739dc8 = []byte{
	// 131 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x28, 0x2e, 0xc9, 0x2f,
	0x4a, 0x2d, 0x2e, 0x48, 0x4d, 0x2e, 0xd6, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x2d, 0x4f,
	0x4d, 0x4a, 0x2b, 0x56, 0x72, 0xe6, 0xe2, 0xf5, 0x08, 0x09, 0x09, 0x08, 0x06, 0x49, 0x07, 0x17,
	0xa4, 0x26, 0x0b, 0x89, 0x71, 0xb1, 0x15, 0x14, 0xa5, 0xa6, 0x65, 0x56, 0x48, 0x30, 0x2a, 0x30,
	0x6a, 0x70, 0x06, 0x41, 0x79, 0x42, 0x52, 0x5c, 0x1c, 0xa9, 0x79, 0x29, 0x05, 0xf9, 0x99, 0x79,
	0x25, 0x12, 0x4c, 0x60, 0x19, 0x38, 0x5f, 0x49, 0x9b, 0x8b, 0xd7, 0x33, 0xc0, 0x2d, 0x18, 0x61,
	0x08, 0xb2, 0x62, 0x46, 0x54, 0xc5, 0x4e, 0x1c, 0x51, 0x6c, 0xb9, 0xf9, 0x29, 0xa9, 0x39, 0xc5,
	0x49, 0x6c, 0x60, 0x97, 0x18, 0x03, 0x02, 0x00, 0x00, 0xff, 0xff, 0xf0, 0x02, 0xc6, 0x39, 0x9d,
	0x00, 0x00, 0x00,
}