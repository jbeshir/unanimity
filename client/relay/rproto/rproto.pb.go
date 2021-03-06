// Code generated by protoc-gen-go.
// source: rproto.proto
// DO NOT EDIT!

package rproto

import proto "code.google.com/p/goprotobuf/proto"
import json "encoding/json"
import math "math"

// Reference proto, json, and math imports to suppress error if they are not otherwise used.
var _ = proto.Marshal
var _ = &json.SyntaxError{}
var _ = math.Inf

type Forward struct {
	Ttl              *uint32 `protobuf:"varint,1,req,name=ttl" json:"ttl,omitempty"`
	Sender           *uint64 `protobuf:"varint,2,req,name=sender" json:"sender,omitempty"`
	Recipient        *uint64 `protobuf:"varint,3,req,name=recipient" json:"recipient,omitempty"`
	Tag              *string `protobuf:"bytes,4,req,name=tag" json:"tag,omitempty"`
	Content          *string `protobuf:"bytes,5,req,name=content" json:"content,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *Forward) Reset()         { *m = Forward{} }
func (m *Forward) String() string { return proto.CompactTextString(m) }
func (*Forward) ProtoMessage()    {}

func (m *Forward) GetTtl() uint32 {
	if m != nil && m.Ttl != nil {
		return *m.Ttl
	}
	return 0
}

func (m *Forward) GetSender() uint64 {
	if m != nil && m.Sender != nil {
		return *m.Sender
	}
	return 0
}

func (m *Forward) GetRecipient() uint64 {
	if m != nil && m.Recipient != nil {
		return *m.Recipient
	}
	return 0
}

func (m *Forward) GetTag() string {
	if m != nil && m.Tag != nil {
		return *m.Tag
	}
	return ""
}

func (m *Forward) GetContent() string {
	if m != nil && m.Content != nil {
		return *m.Content
	}
	return ""
}

func init() {
}
