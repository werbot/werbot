// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.28.2
// source: test.proto

package test

import (
	_ "github.com/werbot/werbot/pkg/utils/protoutils/ghoster/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type MessageTest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ProtoString     string                  `protobuf:"bytes,1,opt,name=proto_string,json=protoString,proto3" json:"proto_string,omitempty"`
	ProtoInt32      int32                   `protobuf:"varint,2,opt,name=proto_int32,json=protoInt32,proto3" json:"proto_int32,omitempty"`
	ProtoInt64      int64                   `protobuf:"varint,3,opt,name=proto_int64,json=protoInt64,proto3" json:"proto_int64,omitempty"`
	ProtoUint32     uint32                  `protobuf:"varint,4,opt,name=proto_uint32,json=protoUint32,proto3" json:"proto_uint32,omitempty"`
	ProtoUint64     uint64                  `protobuf:"varint,5,opt,name=proto_uint64,json=protoUint64,proto3" json:"proto_uint64,omitempty"`
	ProtoFloat      float32                 `protobuf:"fixed32,6,opt,name=proto_float,json=protoFloat,proto3" json:"proto_float,omitempty"`
	ProtoDouble     float64                 `protobuf:"fixed64,7,opt,name=proto_double,json=protoDouble,proto3" json:"proto_double,omitempty"`
	ProtoBool       bool                    `protobuf:"varint,8,opt,name=proto_bool,json=protoBool,proto3" json:"proto_bool,omitempty"`
	ProtoBytes      []byte                  `protobuf:"bytes,9,opt,name=proto_bytes,json=protoBytes,proto3" json:"proto_bytes,omitempty"`
	GoogleTimestamp *timestamppb.Timestamp  `protobuf:"bytes,20,opt,name=google_timestamp,json=googleTimestamp,proto3" json:"google_timestamp,omitempty"`
	GoogleDuration  *durationpb.Duration    `protobuf:"bytes,21,opt,name=google_duration,json=googleDuration,proto3" json:"google_duration,omitempty"`
	GoogleDouble    *wrapperspb.DoubleValue `protobuf:"bytes,22,opt,name=google_double,json=googleDouble,proto3" json:"google_double,omitempty"`
	GoogleFloat     *wrapperspb.FloatValue  `protobuf:"bytes,23,opt,name=google_float,json=googleFloat,proto3" json:"google_float,omitempty"`
	GoogleInt64     *wrapperspb.Int64Value  `protobuf:"bytes,24,opt,name=google_int64,json=googleInt64,proto3" json:"google_int64,omitempty"`
	GoogleUint64    *wrapperspb.UInt64Value `protobuf:"bytes,25,opt,name=google_uint64,json=googleUint64,proto3" json:"google_uint64,omitempty"`
	GoogleInt32     *wrapperspb.Int32Value  `protobuf:"bytes,26,opt,name=google_int32,json=googleInt32,proto3" json:"google_int32,omitempty"`
	GoogleUint32    *wrapperspb.UInt32Value `protobuf:"bytes,27,opt,name=google_uint32,json=googleUint32,proto3" json:"google_uint32,omitempty"`
	GoogleBool      *wrapperspb.BoolValue   `protobuf:"bytes,28,opt,name=google_bool,json=googleBool,proto3" json:"google_bool,omitempty"`
	GoogleString    *wrapperspb.StringValue `protobuf:"bytes,29,opt,name=google_string,json=googleString,proto3" json:"google_string,omitempty"`
	GoogleBytes     *wrapperspb.BytesValue  `protobuf:"bytes,30,opt,name=google_bytes,json=googleBytes,proto3" json:"google_bytes,omitempty"`
}

func (x *MessageTest) Reset() {
	*x = MessageTest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MessageTest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MessageTest) ProtoMessage() {}

func (x *MessageTest) ProtoReflect() protoreflect.Message {
	mi := &file_test_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MessageTest.ProtoReflect.Descriptor instead.
func (*MessageTest) Descriptor() ([]byte, []int) {
	return file_test_proto_rawDescGZIP(), []int{0}
}

func (x *MessageTest) GetProtoString() string {
	if x != nil {
		return x.ProtoString
	}
	return ""
}

func (x *MessageTest) GetProtoInt32() int32 {
	if x != nil {
		return x.ProtoInt32
	}
	return 0
}

func (x *MessageTest) GetProtoInt64() int64 {
	if x != nil {
		return x.ProtoInt64
	}
	return 0
}

func (x *MessageTest) GetProtoUint32() uint32 {
	if x != nil {
		return x.ProtoUint32
	}
	return 0
}

func (x *MessageTest) GetProtoUint64() uint64 {
	if x != nil {
		return x.ProtoUint64
	}
	return 0
}

func (x *MessageTest) GetProtoFloat() float32 {
	if x != nil {
		return x.ProtoFloat
	}
	return 0
}

func (x *MessageTest) GetProtoDouble() float64 {
	if x != nil {
		return x.ProtoDouble
	}
	return 0
}

func (x *MessageTest) GetProtoBool() bool {
	if x != nil {
		return x.ProtoBool
	}
	return false
}

func (x *MessageTest) GetProtoBytes() []byte {
	if x != nil {
		return x.ProtoBytes
	}
	return nil
}

func (x *MessageTest) GetGoogleTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.GoogleTimestamp
	}
	return nil
}

func (x *MessageTest) GetGoogleDuration() *durationpb.Duration {
	if x != nil {
		return x.GoogleDuration
	}
	return nil
}

func (x *MessageTest) GetGoogleDouble() *wrapperspb.DoubleValue {
	if x != nil {
		return x.GoogleDouble
	}
	return nil
}

func (x *MessageTest) GetGoogleFloat() *wrapperspb.FloatValue {
	if x != nil {
		return x.GoogleFloat
	}
	return nil
}

func (x *MessageTest) GetGoogleInt64() *wrapperspb.Int64Value {
	if x != nil {
		return x.GoogleInt64
	}
	return nil
}

func (x *MessageTest) GetGoogleUint64() *wrapperspb.UInt64Value {
	if x != nil {
		return x.GoogleUint64
	}
	return nil
}

func (x *MessageTest) GetGoogleInt32() *wrapperspb.Int32Value {
	if x != nil {
		return x.GoogleInt32
	}
	return nil
}

func (x *MessageTest) GetGoogleUint32() *wrapperspb.UInt32Value {
	if x != nil {
		return x.GoogleUint32
	}
	return nil
}

func (x *MessageTest) GetGoogleBool() *wrapperspb.BoolValue {
	if x != nil {
		return x.GoogleBool
	}
	return nil
}

func (x *MessageTest) GetGoogleString() *wrapperspb.StringValue {
	if x != nil {
		return x.GoogleString
	}
	return nil
}

func (x *MessageTest) GetGoogleBytes() *wrapperspb.BytesValue {
	if x != nil {
		return x.GoogleBytes
	}
	return nil
}

var File_test_proto protoreflect.FileDescriptor

var file_test_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x67, 0x68,
	0x6f, 0x73, 0x74, 0x65, 0x72, 0x1a, 0x30, 0x70, 0x6b, 0x67, 0x2f, 0x75, 0x74, 0x69, 0x6c, 0x73,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x75, 0x74, 0x69, 0x6c, 0x73, 0x2f, 0x67, 0x68, 0x6f, 0x73,
	0x74, 0x65, 0x72, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x68, 0x6f, 0x73, 0x74, 0x65,
	0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x77, 0x72, 0x61, 0x70, 0x70, 0x65,
	0x72, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x88, 0x09, 0x0a, 0x0b, 0x4d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x54, 0x65, 0x73, 0x74, 0x12, 0x27, 0x0a, 0x0c, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x5f, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x04,
	0x88, 0xb5, 0x18, 0x01, 0x52, 0x0b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x53, 0x74, 0x72, 0x69, 0x6e,
	0x67, 0x12, 0x25, 0x0a, 0x0b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x69, 0x6e, 0x74, 0x33, 0x32,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52, 0x0a, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x49, 0x6e, 0x74, 0x33, 0x32, 0x12, 0x25, 0x0a, 0x0b, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x5f, 0x69, 0x6e, 0x74, 0x36, 0x34, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x42, 0x04, 0x88,
	0xb5, 0x18, 0x01, 0x52, 0x0a, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x49, 0x6e, 0x74, 0x36, 0x34, 0x12,
	0x27, 0x0a, 0x0c, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x75, 0x69, 0x6e, 0x74, 0x33, 0x32, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x0d, 0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52, 0x0b, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x55, 0x69, 0x6e, 0x74, 0x33, 0x32, 0x12, 0x27, 0x0a, 0x0c, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x5f, 0x75, 0x69, 0x6e, 0x74, 0x36, 0x34, 0x18, 0x05, 0x20, 0x01, 0x28, 0x04, 0x42, 0x04,
	0x88, 0xb5, 0x18, 0x01, 0x52, 0x0b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x55, 0x69, 0x6e, 0x74, 0x36,
	0x34, 0x12, 0x25, 0x0a, 0x0b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x66, 0x6c, 0x6f, 0x61, 0x74,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x02, 0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52, 0x0a, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x46, 0x6c, 0x6f, 0x61, 0x74, 0x12, 0x27, 0x0a, 0x0c, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x5f, 0x64, 0x6f, 0x75, 0x62, 0x6c, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x01, 0x42, 0x04,
	0x88, 0xb5, 0x18, 0x01, 0x52, 0x0b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x44, 0x6f, 0x75, 0x62, 0x6c,
	0x65, 0x12, 0x23, 0x0a, 0x0a, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x62, 0x6f, 0x6f, 0x6c, 0x18,
	0x08, 0x20, 0x01, 0x28, 0x08, 0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52, 0x09, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x42, 0x6f, 0x6f, 0x6c, 0x12, 0x25, 0x0a, 0x0b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f,
	0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x04, 0x88, 0xb5, 0x18,
	0x01, 0x52, 0x0a, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x42, 0x79, 0x74, 0x65, 0x73, 0x12, 0x4b, 0x0a,
	0x10, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x18, 0x14, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74,
	0x61, 0x6d, 0x70, 0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52, 0x0f, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x48, 0x0a, 0x0f, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x5f, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x15, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x04,
	0x88, 0xb5, 0x18, 0x01, 0x52, 0x0e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x44, 0x75, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x12, 0x47, 0x0a, 0x0d, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x5f, 0x64,
	0x6f, 0x75, 0x62, 0x6c, 0x65, 0x18, 0x16, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x6f,
	0x75, 0x62, 0x6c, 0x65, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52,
	0x0c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x44, 0x6f, 0x75, 0x62, 0x6c, 0x65, 0x12, 0x44, 0x0a,
	0x0c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x5f, 0x66, 0x6c, 0x6f, 0x61, 0x74, 0x18, 0x17, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x46, 0x6c, 0x6f, 0x61, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65,
	0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52, 0x0b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x46, 0x6c,
	0x6f, 0x61, 0x74, 0x12, 0x44, 0x0a, 0x0c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x5f, 0x69, 0x6e,
	0x74, 0x36, 0x34, 0x18, 0x18, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x49, 0x6e, 0x74, 0x36,
	0x34, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52, 0x0b, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x49, 0x6e, 0x74, 0x36, 0x34, 0x12, 0x47, 0x0a, 0x0d, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x5f, 0x75, 0x69, 0x6e, 0x74, 0x36, 0x34, 0x18, 0x19, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1c, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x55, 0x49, 0x6e, 0x74, 0x36, 0x34, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x04,
	0x88, 0xb5, 0x18, 0x01, 0x52, 0x0c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x55, 0x69, 0x6e, 0x74,
	0x36, 0x34, 0x12, 0x44, 0x0a, 0x0c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x5f, 0x69, 0x6e, 0x74,
	0x33, 0x32, 0x18, 0x1a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x49, 0x6e, 0x74, 0x33, 0x32,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52, 0x0b, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x49, 0x6e, 0x74, 0x33, 0x32, 0x12, 0x47, 0x0a, 0x0d, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x5f, 0x75, 0x69, 0x6e, 0x74, 0x33, 0x32, 0x18, 0x1b, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x1c, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x55, 0x49, 0x6e, 0x74, 0x33, 0x32, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x04, 0x88,
	0xb5, 0x18, 0x01, 0x52, 0x0c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x55, 0x69, 0x6e, 0x74, 0x33,
	0x32, 0x12, 0x41, 0x0a, 0x0b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x5f, 0x62, 0x6f, 0x6f, 0x6c,
	0x18, 0x1c, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x42, 0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52, 0x0a, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x42, 0x6f, 0x6f, 0x6c, 0x12, 0x47, 0x0a, 0x0d, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x5f, 0x73,
	0x74, 0x72, 0x69, 0x6e, 0x67, 0x18, 0x1d, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x53, 0x74,
	0x72, 0x69, 0x6e, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52,
	0x0c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x53, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x12, 0x44, 0x0a,
	0x0c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x5f, 0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x1e, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x42, 0x79, 0x74, 0x65, 0x73, 0x56, 0x61, 0x6c, 0x75, 0x65,
	0x42, 0x04, 0x88, 0xb5, 0x18, 0x01, 0x52, 0x0b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x42, 0x79,
	0x74, 0x65, 0x73, 0x42, 0x41, 0x5a, 0x3f, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f,
	0x70, 0x6b, 0x67, 0x75, 0x74, 0x69, 0x6c, 0x73, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x75, 0x74,
	0x69, 0x6c, 0x73, 0x2f, 0x67, 0x68, 0x6f, 0x73, 0x74, 0x65, 0x72, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_test_proto_rawDescOnce sync.Once
	file_test_proto_rawDescData = file_test_proto_rawDesc
)

func file_test_proto_rawDescGZIP() []byte {
	file_test_proto_rawDescOnce.Do(func() {
		file_test_proto_rawDescData = protoimpl.X.CompressGZIP(file_test_proto_rawDescData)
	})
	return file_test_proto_rawDescData
}

var file_test_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_test_proto_goTypes = []any{
	(*MessageTest)(nil),            // 0: ghoster.MessageTest
	(*timestamppb.Timestamp)(nil),  // 1: google.protobuf.Timestamp
	(*durationpb.Duration)(nil),    // 2: google.protobuf.Duration
	(*wrapperspb.DoubleValue)(nil), // 3: google.protobuf.DoubleValue
	(*wrapperspb.FloatValue)(nil),  // 4: google.protobuf.FloatValue
	(*wrapperspb.Int64Value)(nil),  // 5: google.protobuf.Int64Value
	(*wrapperspb.UInt64Value)(nil), // 6: google.protobuf.UInt64Value
	(*wrapperspb.Int32Value)(nil),  // 7: google.protobuf.Int32Value
	(*wrapperspb.UInt32Value)(nil), // 8: google.protobuf.UInt32Value
	(*wrapperspb.BoolValue)(nil),   // 9: google.protobuf.BoolValue
	(*wrapperspb.StringValue)(nil), // 10: google.protobuf.StringValue
	(*wrapperspb.BytesValue)(nil),  // 11: google.protobuf.BytesValue
}
var file_test_proto_depIdxs = []int32{
	1,  // 0: ghoster.MessageTest.google_timestamp:type_name -> google.protobuf.Timestamp
	2,  // 1: ghoster.MessageTest.google_duration:type_name -> google.protobuf.Duration
	3,  // 2: ghoster.MessageTest.google_double:type_name -> google.protobuf.DoubleValue
	4,  // 3: ghoster.MessageTest.google_float:type_name -> google.protobuf.FloatValue
	5,  // 4: ghoster.MessageTest.google_int64:type_name -> google.protobuf.Int64Value
	6,  // 5: ghoster.MessageTest.google_uint64:type_name -> google.protobuf.UInt64Value
	7,  // 6: ghoster.MessageTest.google_int32:type_name -> google.protobuf.Int32Value
	8,  // 7: ghoster.MessageTest.google_uint32:type_name -> google.protobuf.UInt32Value
	9,  // 8: ghoster.MessageTest.google_bool:type_name -> google.protobuf.BoolValue
	10, // 9: ghoster.MessageTest.google_string:type_name -> google.protobuf.StringValue
	11, // 10: ghoster.MessageTest.google_bytes:type_name -> google.protobuf.BytesValue
	11, // [11:11] is the sub-list for method output_type
	11, // [11:11] is the sub-list for method input_type
	11, // [11:11] is the sub-list for extension type_name
	11, // [11:11] is the sub-list for extension extendee
	0,  // [0:11] is the sub-list for field type_name
}

func init() { file_test_proto_init() }
func file_test_proto_init() {
	if File_test_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_test_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*MessageTest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_test_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_test_proto_goTypes,
		DependencyIndexes: file_test_proto_depIdxs,
		MessageInfos:      file_test_proto_msgTypes,
	}.Build()
	File_test_proto = out.File
	file_test_proto_rawDesc = nil
	file_test_proto_goTypes = nil
	file_test_proto_depIdxs = nil
}
