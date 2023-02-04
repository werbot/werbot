// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.12
// source: logging.proto

package logging

import (
	_ "github.com/envoyproxy/protoc-gen-validate/validate"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Logger int32

const (
	Logger_loger_unspecified Logger = 0
	Logger_profile           Logger = 1
	Logger_project           Logger = 2
	Logger_server            Logger = 3
)

// Enum value maps for Logger.
var (
	Logger_name = map[int32]string{
		0: "loger_unspecified",
		1: "profile",
		2: "project",
		3: "server",
	}
	Logger_value = map[string]int32{
		"loger_unspecified": 0,
		"profile":           1,
		"project":           2,
		"server":            3,
	}
)

func (x Logger) Enum() *Logger {
	p := new(Logger)
	*p = x
	return p
}

func (x Logger) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Logger) Descriptor() protoreflect.EnumDescriptor {
	return file_logging_proto_enumTypes[0].Descriptor()
}

func (Logger) Type() protoreflect.EnumType {
	return &file_logging_proto_enumTypes[0]
}

func (x Logger) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Logger.Descriptor instead.
func (Logger) EnumDescriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{0}
}

type EventType int32

const (
	EventType_event_unspecified EventType = 0
	EventType_onOnline          EventType = 1
	EventType_onOffline         EventType = 2
	EventType_onCreate          EventType = 3
	EventType_onEdit            EventType = 4
	EventType_onRemove          EventType = 5
	EventType_onActive          EventType = 6
	EventType_onInactive        EventType = 7
	EventType_onChange          EventType = 8
)

// Enum value maps for EventType.
var (
	EventType_name = map[int32]string{
		0: "event_unspecified",
		1: "onOnline",
		2: "onOffline",
		3: "onCreate",
		4: "onEdit",
		5: "onRemove",
		6: "onActive",
		7: "onInactive",
		8: "onChange",
	}
	EventType_value = map[string]int32{
		"event_unspecified": 0,
		"onOnline":          1,
		"onOffline":         2,
		"onCreate":          3,
		"onEdit":            4,
		"onRemove":          5,
		"onActive":          6,
		"onInactive":        7,
		"onChange":          8,
	}
)

func (x EventType) Enum() *EventType {
	p := new(EventType)
	*p = x
	return p
}

func (x EventType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (EventType) Descriptor() protoreflect.EnumDescriptor {
	return file_logging_proto_enumTypes[1].Descriptor()
}

func (EventType) Type() protoreflect.EnumType {
	return &file_logging_proto_enumTypes[1]
}

func (x EventType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use EventType.Descriptor instead.
func (EventType) EnumDescriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{1}
}

// rpc ListRecords
type ListRecords struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ListRecords) Reset() {
	*x = ListRecords{}
	if protoimpl.UnsafeEnabled {
		mi := &file_logging_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListRecords) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRecords) ProtoMessage() {}

func (x *ListRecords) ProtoReflect() protoreflect.Message {
	mi := &file_logging_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRecords.ProtoReflect.Descriptor instead.
func (*ListRecords) Descriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{0}
}

// rpc Record
type Record struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Record) Reset() {
	*x = Record{}
	if protoimpl.UnsafeEnabled {
		mi := &file_logging_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Record) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Record) ProtoMessage() {}

func (x *Record) ProtoReflect() protoreflect.Message {
	mi := &file_logging_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Record.ProtoReflect.Descriptor instead.
func (*Record) Descriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{1}
}

// rpc AddRecord
type AddRecord struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *AddRecord) Reset() {
	*x = AddRecord{}
	if protoimpl.UnsafeEnabled {
		mi := &file_logging_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AddRecord) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddRecord) ProtoMessage() {}

func (x *AddRecord) ProtoReflect() protoreflect.Message {
	mi := &file_logging_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddRecord.ProtoReflect.Descriptor instead.
func (*AddRecord) Descriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{2}
}

type ListRecords_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Logger Logger `protobuf:"varint,1,opt,name=logger,proto3,enum=logging.Logger" json:"logger,omitempty"`
	Id     string `protobuf:"bytes,2,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *ListRecords_Request) Reset() {
	*x = ListRecords_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_logging_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListRecords_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRecords_Request) ProtoMessage() {}

func (x *ListRecords_Request) ProtoReflect() protoreflect.Message {
	mi := &file_logging_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRecords_Request.ProtoReflect.Descriptor instead.
func (*ListRecords_Request) Descriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{0, 0}
}

func (x *ListRecords_Request) GetLogger() Logger {
	if x != nil {
		return x.Logger
	}
	return Logger_loger_unspecified
}

func (x *ListRecords_Request) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type ListRecords_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ListRecords_Response) Reset() {
	*x = ListRecords_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_logging_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListRecords_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRecords_Response) ProtoMessage() {}

func (x *ListRecords_Response) ProtoReflect() protoreflect.Message {
	mi := &file_logging_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRecords_Response.ProtoReflect.Descriptor instead.
func (*ListRecords_Response) Descriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{0, 1}
}

type Record_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RecordId string `protobuf:"bytes,3,opt,name=record_id,json=recordId,proto3" json:"record_id,omitempty"`
}

func (x *Record_Request) Reset() {
	*x = Record_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_logging_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Record_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Record_Request) ProtoMessage() {}

func (x *Record_Request) ProtoReflect() protoreflect.Message {
	mi := &file_logging_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Record_Request.ProtoReflect.Descriptor instead.
func (*Record_Request) Descriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{1, 0}
}

func (x *Record_Request) GetRecordId() string {
	if x != nil {
		return x.RecordId
	}
	return ""
}

type Record_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Record_Response) Reset() {
	*x = Record_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_logging_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Record_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Record_Response) ProtoMessage() {}

func (x *Record_Response) ProtoReflect() protoreflect.Message {
	mi := &file_logging_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Record_Response.ProtoReflect.Descriptor instead.
func (*Record_Response) Descriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{1, 1}
}

type AddRecord_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Logger Logger    `protobuf:"varint,1,opt,name=logger,proto3,enum=logging.Logger" json:"logger,omitempty"`
	Event  EventType `protobuf:"varint,2,opt,name=event,proto3,enum=logging.EventType" json:"event,omitempty"`
	Id     string    `protobuf:"bytes,3,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *AddRecord_Request) Reset() {
	*x = AddRecord_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_logging_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AddRecord_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddRecord_Request) ProtoMessage() {}

func (x *AddRecord_Request) ProtoReflect() protoreflect.Message {
	mi := &file_logging_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddRecord_Request.ProtoReflect.Descriptor instead.
func (*AddRecord_Request) Descriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{2, 0}
}

func (x *AddRecord_Request) GetLogger() Logger {
	if x != nil {
		return x.Logger
	}
	return Logger_loger_unspecified
}

func (x *AddRecord_Request) GetEvent() EventType {
	if x != nil {
		return x.Event
	}
	return EventType_event_unspecified
}

func (x *AddRecord_Request) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type AddRecord_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *AddRecord_Response) Reset() {
	*x = AddRecord_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_logging_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AddRecord_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddRecord_Response) ProtoMessage() {}

func (x *AddRecord_Response) ProtoReflect() protoreflect.Message {
	mi := &file_logging_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddRecord_Response.ProtoReflect.Descriptor instead.
func (*AddRecord_Response) Descriptor() ([]byte, []int) {
	return file_logging_proto_rawDescGZIP(), []int{2, 1}
}

var File_logging_proto protoreflect.FileDescriptor

var file_logging_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x6c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x07, 0x6c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x1a, 0x17, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61,
	0x74, 0x65, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0x71, 0x0a, 0x0b, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x73,
	0x1a, 0x56, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x31, 0x0a, 0x06, 0x6c,
	0x6f, 0x67, 0x67, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0f, 0x2e, 0x6c, 0x6f,
	0x67, 0x67, 0x69, 0x6e, 0x67, 0x2e, 0x4c, 0x6f, 0x67, 0x67, 0x65, 0x72, 0x42, 0x08, 0xfa, 0x42,
	0x05, 0x82, 0x01, 0x02, 0x10, 0x01, 0x52, 0x06, 0x6c, 0x6f, 0x67, 0x67, 0x65, 0x72, 0x12, 0x18,
	0x0a, 0x02, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x08, 0xfa, 0x42, 0x05, 0x72,
	0x03, 0xb0, 0x01, 0x01, 0x52, 0x02, 0x69, 0x64, 0x1a, 0x0a, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x22, 0x46, 0x0a, 0x06, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x1a, 0x30,
	0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x25, 0x0a, 0x09, 0x72, 0x65, 0x63,
	0x6f, 0x72, 0x64, 0x5f, 0x69, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x42, 0x08, 0xfa, 0x42,
	0x05, 0x72, 0x03, 0xb0, 0x01, 0x01, 0x52, 0x08, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x49, 0x64,
	0x1a, 0x0a, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0xa4, 0x01, 0x0a,
	0x09, 0x41, 0x64, 0x64, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x1a, 0x8a, 0x01, 0x0a, 0x07, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x31, 0x0a, 0x06, 0x6c, 0x6f, 0x67, 0x67, 0x65, 0x72,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0f, 0x2e, 0x6c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67,
	0x2e, 0x4c, 0x6f, 0x67, 0x67, 0x65, 0x72, 0x42, 0x08, 0xfa, 0x42, 0x05, 0x82, 0x01, 0x02, 0x10,
	0x01, 0x52, 0x06, 0x6c, 0x6f, 0x67, 0x67, 0x65, 0x72, 0x12, 0x32, 0x0a, 0x05, 0x65, 0x76, 0x65,
	0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x12, 0x2e, 0x6c, 0x6f, 0x67, 0x67, 0x69,
	0x6e, 0x67, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x54, 0x79, 0x70, 0x65, 0x42, 0x08, 0xfa, 0x42,
	0x05, 0x82, 0x01, 0x02, 0x10, 0x01, 0x52, 0x05, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x12, 0x18, 0x0a,
	0x02, 0x69, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x42, 0x08, 0xfa, 0x42, 0x05, 0x72, 0x03,
	0xb0, 0x01, 0x01, 0x52, 0x02, 0x69, 0x64, 0x1a, 0x0a, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x2a, 0x45, 0x0a, 0x06, 0x4c, 0x6f, 0x67, 0x67, 0x65, 0x72, 0x12, 0x15, 0x0a,
	0x11, 0x6c, 0x6f, 0x67, 0x65, 0x72, 0x5f, 0x75, 0x6e, 0x73, 0x70, 0x65, 0x63, 0x69, 0x66, 0x69,
	0x65, 0x64, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x70, 0x72, 0x6f, 0x66, 0x69, 0x6c, 0x65, 0x10,
	0x01, 0x12, 0x0b, 0x0a, 0x07, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x10, 0x02, 0x12, 0x0a,
	0x0a, 0x06, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x10, 0x03, 0x2a, 0x93, 0x01, 0x0a, 0x09, 0x45,
	0x76, 0x65, 0x6e, 0x74, 0x54, 0x79, 0x70, 0x65, 0x12, 0x15, 0x0a, 0x11, 0x65, 0x76, 0x65, 0x6e,
	0x74, 0x5f, 0x75, 0x6e, 0x73, 0x70, 0x65, 0x63, 0x69, 0x66, 0x69, 0x65, 0x64, 0x10, 0x00, 0x12,
	0x0c, 0x0a, 0x08, 0x6f, 0x6e, 0x4f, 0x6e, 0x6c, 0x69, 0x6e, 0x65, 0x10, 0x01, 0x12, 0x0d, 0x0a,
	0x09, 0x6f, 0x6e, 0x4f, 0x66, 0x66, 0x6c, 0x69, 0x6e, 0x65, 0x10, 0x02, 0x12, 0x0c, 0x0a, 0x08,
	0x6f, 0x6e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x10, 0x03, 0x12, 0x0a, 0x0a, 0x06, 0x6f, 0x6e,
	0x45, 0x64, 0x69, 0x74, 0x10, 0x04, 0x12, 0x0c, 0x0a, 0x08, 0x6f, 0x6e, 0x52, 0x65, 0x6d, 0x6f,
	0x76, 0x65, 0x10, 0x05, 0x12, 0x0c, 0x0a, 0x08, 0x6f, 0x6e, 0x41, 0x63, 0x74, 0x69, 0x76, 0x65,
	0x10, 0x06, 0x12, 0x0e, 0x0a, 0x0a, 0x6f, 0x6e, 0x49, 0x6e, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65,
	0x10, 0x07, 0x12, 0x0c, 0x0a, 0x08, 0x6f, 0x6e, 0x43, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x10, 0x08,
	0x32, 0xe6, 0x01, 0x0a, 0x0f, 0x4c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x48, 0x61, 0x6e, 0x64,
	0x6c, 0x65, 0x72, 0x73, 0x12, 0x4c, 0x0a, 0x0b, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x63, 0x6f,
	0x72, 0x64, 0x73, 0x12, 0x1c, 0x2e, 0x6c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x2e, 0x4c, 0x69,
	0x73, 0x74, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x73, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x1d, 0x2e, 0x6c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x2e, 0x4c, 0x69, 0x73, 0x74,
	0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x73, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x22, 0x00, 0x12, 0x3d, 0x0a, 0x06, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x12, 0x17, 0x2e, 0x6c,
	0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x2e, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x2e, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x6c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x2e,
	0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x46, 0x0a, 0x09, 0x41, 0x64, 0x64, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x12, 0x1a,
	0x2e, 0x6c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x2e, 0x41, 0x64, 0x64, 0x52, 0x65, 0x63, 0x6f,
	0x72, 0x64, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1b, 0x2e, 0x6c, 0x6f, 0x67,
	0x67, 0x69, 0x6e, 0x67, 0x2e, 0x41, 0x64, 0x64, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x2e, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x2c, 0x5a, 0x2a, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x77,
	0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f,
	0x6c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_logging_proto_rawDescOnce sync.Once
	file_logging_proto_rawDescData = file_logging_proto_rawDesc
)

func file_logging_proto_rawDescGZIP() []byte {
	file_logging_proto_rawDescOnce.Do(func() {
		file_logging_proto_rawDescData = protoimpl.X.CompressGZIP(file_logging_proto_rawDescData)
	})
	return file_logging_proto_rawDescData
}

var file_logging_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_logging_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_logging_proto_goTypes = []interface{}{
	(Logger)(0),                  // 0: logging.Logger
	(EventType)(0),               // 1: logging.EventType
	(*ListRecords)(nil),          // 2: logging.ListRecords
	(*Record)(nil),               // 3: logging.Record
	(*AddRecord)(nil),            // 4: logging.AddRecord
	(*ListRecords_Request)(nil),  // 5: logging.ListRecords.Request
	(*ListRecords_Response)(nil), // 6: logging.ListRecords.Response
	(*Record_Request)(nil),       // 7: logging.Record.Request
	(*Record_Response)(nil),      // 8: logging.Record.Response
	(*AddRecord_Request)(nil),    // 9: logging.AddRecord.Request
	(*AddRecord_Response)(nil),   // 10: logging.AddRecord.Response
}
var file_logging_proto_depIdxs = []int32{
	0,  // 0: logging.ListRecords.Request.logger:type_name -> logging.Logger
	0,  // 1: logging.AddRecord.Request.logger:type_name -> logging.Logger
	1,  // 2: logging.AddRecord.Request.event:type_name -> logging.EventType
	5,  // 3: logging.LoggingHandlers.ListRecords:input_type -> logging.ListRecords.Request
	7,  // 4: logging.LoggingHandlers.Record:input_type -> logging.Record.Request
	9,  // 5: logging.LoggingHandlers.AddRecord:input_type -> logging.AddRecord.Request
	6,  // 6: logging.LoggingHandlers.ListRecords:output_type -> logging.ListRecords.Response
	8,  // 7: logging.LoggingHandlers.Record:output_type -> logging.Record.Response
	10, // 8: logging.LoggingHandlers.AddRecord:output_type -> logging.AddRecord.Response
	6,  // [6:9] is the sub-list for method output_type
	3,  // [3:6] is the sub-list for method input_type
	3,  // [3:3] is the sub-list for extension type_name
	3,  // [3:3] is the sub-list for extension extendee
	0,  // [0:3] is the sub-list for field type_name
}

func init() { file_logging_proto_init() }
func file_logging_proto_init() {
	if File_logging_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_logging_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListRecords); i {
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
		file_logging_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Record); i {
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
		file_logging_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AddRecord); i {
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
		file_logging_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListRecords_Request); i {
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
		file_logging_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListRecords_Response); i {
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
		file_logging_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Record_Request); i {
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
		file_logging_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Record_Response); i {
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
		file_logging_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AddRecord_Request); i {
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
		file_logging_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AddRecord_Response); i {
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
			RawDescriptor: file_logging_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_logging_proto_goTypes,
		DependencyIndexes: file_logging_proto_depIdxs,
		EnumInfos:         file_logging_proto_enumTypes,
		MessageInfos:      file_logging_proto_msgTypes,
	}.Build()
	File_logging_proto = out.File
	file_logging_proto_rawDesc = nil
	file_logging_proto_goTypes = nil
	file_logging_proto_depIdxs = nil
}
