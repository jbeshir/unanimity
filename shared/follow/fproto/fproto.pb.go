// Code generated by protoc-gen-go.
// source: fproto.proto
// DO NOT EDIT!

package fproto

import proto "code.google.com/p/goprotobuf/proto"
import json "encoding/json"
import math "math"

// Reference proto, json, and math imports to suppress error if they are not otherwise used.
var _ = proto.Marshal
var _ = &json.SyntaxError{}
var _ = math.Inf

type Change struct {
	TargetEntity     *uint64 `protobuf:"varint,1,req,name=targetEntity" json:"targetEntity,omitempty"`
	Key              *string `protobuf:"bytes,2,req,name=key" json:"key,omitempty"`
	Value            *string `protobuf:"bytes,3,req,name=value" json:"value,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *Change) Reset()         { *m = Change{} }
func (m *Change) String() string { return proto.CompactTextString(m) }
func (*Change) ProtoMessage()    {}

func (m *Change) GetTargetEntity() uint64 {
	if m != nil && m.TargetEntity != nil {
		return *m.TargetEntity
	}
	return 0
}

func (m *Change) GetKey() string {
	if m != nil && m.Key != nil {
		return *m.Key
	}
	return ""
}

func (m *Change) GetValue() string {
	if m != nil && m.Value != nil {
		return *m.Value
	}
	return ""
}

type ChangeRequest struct {
	RequestEntity    *uint64   `protobuf:"varint,1,req,name=requestEntity" json:"requestEntity,omitempty"`
	RequestNode      *uint32   `protobuf:"varint,2,req,name=requestNode" json:"requestNode,omitempty"`
	RequestId        *uint64   `protobuf:"varint,3,req,name=requestId" json:"requestId,omitempty"`
	Changeset        []*Change `protobuf:"bytes,4,rep,name=changeset" json:"changeset,omitempty"`
	XXX_unrecognized []byte    `json:"-"`
}

func (m *ChangeRequest) Reset()         { *m = ChangeRequest{} }
func (m *ChangeRequest) String() string { return proto.CompactTextString(m) }
func (*ChangeRequest) ProtoMessage()    {}

func (m *ChangeRequest) GetRequestEntity() uint64 {
	if m != nil && m.RequestEntity != nil {
		return *m.RequestEntity
	}
	return 0
}

func (m *ChangeRequest) GetRequestNode() uint32 {
	if m != nil && m.RequestNode != nil {
		return *m.RequestNode
	}
	return 0
}

func (m *ChangeRequest) GetRequestId() uint64 {
	if m != nil && m.RequestId != nil {
		return *m.RequestId
	}
	return 0
}

func (m *ChangeRequest) GetChangeset() []*Change {
	if m != nil {
		return m.Changeset
	}
	return nil
}

type Position struct {
	FirstUnapplied   *uint64 `protobuf:"varint,1,req,name=firstUnapplied" json:"firstUnapplied,omitempty"`
	Degraded         *bool   `protobuf:"varint,2,req,name=degraded" json:"degraded,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *Position) Reset()         { *m = Position{} }
func (m *Position) String() string { return proto.CompactTextString(m) }
func (*Position) ProtoMessage()    {}

func (m *Position) GetFirstUnapplied() uint64 {
	if m != nil && m.FirstUnapplied != nil {
		return *m.FirstUnapplied
	}
	return 0
}

func (m *Position) GetDegraded() bool {
	if m != nil && m.Degraded != nil {
		return *m.Degraded
	}
	return false
}

type GlobalProperty struct {
	Key              *string `protobuf:"bytes,1,req,name=key" json:"key,omitempty"`
	Value            *string `protobuf:"bytes,2,req,name=value" json:"value,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *GlobalProperty) Reset()         { *m = GlobalProperty{} }
func (m *GlobalProperty) String() string { return proto.CompactTextString(m) }
func (*GlobalProperty) ProtoMessage()    {}

func (m *GlobalProperty) GetKey() string {
	if m != nil && m.Key != nil {
		return *m.Key
	}
	return ""
}

func (m *GlobalProperty) GetValue() string {
	if m != nil && m.Value != nil {
		return *m.Value
	}
	return ""
}

type EntityProperty struct {
	Entity           *uint64 `protobuf:"varint,1,req,name=entity" json:"entity,omitempty"`
	Key              *string `protobuf:"bytes,2,req,name=key" json:"key,omitempty"`
	Value            *string `protobuf:"bytes,3,req,name=value" json:"value,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *EntityProperty) Reset()         { *m = EntityProperty{} }
func (m *EntityProperty) String() string { return proto.CompactTextString(m) }
func (*EntityProperty) ProtoMessage()    {}

func (m *EntityProperty) GetEntity() uint64 {
	if m != nil && m.Entity != nil {
		return *m.Entity
	}
	return 0
}

func (m *EntityProperty) GetKey() string {
	if m != nil && m.Key != nil {
		return *m.Key
	}
	return ""
}

func (m *EntityProperty) GetValue() string {
	if m != nil && m.Value != nil {
		return *m.Value
	}
	return ""
}

type BurstDone struct {
	FirstUnapplied   *uint64 `protobuf:"varint,1,req,name=firstUnapplied" json:"firstUnapplied,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *BurstDone) Reset()         { *m = BurstDone{} }
func (m *BurstDone) String() string { return proto.CompactTextString(m) }
func (*BurstDone) ProtoMessage()    {}

func (m *BurstDone) GetFirstUnapplied() uint64 {
	if m != nil && m.FirstUnapplied != nil {
		return *m.FirstUnapplied
	}
	return 0
}

type InstructionChosen struct {
	Slot             *uint64 `protobuf:"varint,1,req,name=slot" json:"slot,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *InstructionChosen) Reset()         { *m = InstructionChosen{} }
func (m *InstructionChosen) String() string { return proto.CompactTextString(m) }
func (*InstructionChosen) ProtoMessage()    {}

func (m *InstructionChosen) GetSlot() uint64 {
	if m != nil && m.Slot != nil {
		return *m.Slot
	}
	return 0
}

type InstructionRequest struct {
	Slot             *uint64 `protobuf:"varint,1,req,name=slot" json:"slot,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *InstructionRequest) Reset()         { *m = InstructionRequest{} }
func (m *InstructionRequest) String() string { return proto.CompactTextString(m) }
func (*InstructionRequest) ProtoMessage()    {}

func (m *InstructionRequest) GetSlot() uint64 {
	if m != nil && m.Slot != nil {
		return *m.Slot
	}
	return 0
}

type InstructionData struct {
	Slot             *uint64        `protobuf:"varint,1,req,name=slot" json:"slot,omitempty"`
	Request          *ChangeRequest `protobuf:"bytes,4,req,name=request" json:"request,omitempty"`
	XXX_unrecognized []byte         `json:"-"`
}

func (m *InstructionData) Reset()         { *m = InstructionData{} }
func (m *InstructionData) String() string { return proto.CompactTextString(m) }
func (*InstructionData) ProtoMessage()    {}

func (m *InstructionData) GetSlot() uint64 {
	if m != nil && m.Slot != nil {
		return *m.Slot
	}
	return 0
}

func (m *InstructionData) GetRequest() *ChangeRequest {
	if m != nil {
		return m.Request
	}
	return nil
}

type FirstUnapplied struct {
	FirstUnapplied   *uint64 `protobuf:"varint,1,req,name=firstUnapplied" json:"firstUnapplied,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *FirstUnapplied) Reset()         { *m = FirstUnapplied{} }
func (m *FirstUnapplied) String() string { return proto.CompactTextString(m) }
func (*FirstUnapplied) ProtoMessage()    {}

func (m *FirstUnapplied) GetFirstUnapplied() uint64 {
	if m != nil && m.FirstUnapplied != nil {
		return *m.FirstUnapplied
	}
	return 0
}

type Leader struct {
	Proposal         *uint64 `protobuf:"varint,1,req,name=proposal" json:"proposal,omitempty"`
	Leader           *uint32 `protobuf:"varint,2,req,name=leader" json:"leader,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *Leader) Reset()         { *m = Leader{} }
func (m *Leader) String() string { return proto.CompactTextString(m) }
func (*Leader) ProtoMessage()    {}

func (m *Leader) GetProposal() uint64 {
	if m != nil && m.Proposal != nil {
		return *m.Proposal
	}
	return 0
}

func (m *Leader) GetLeader() uint32 {
	if m != nil && m.Leader != nil {
		return *m.Leader
	}
	return 0
}

func init() {
}
