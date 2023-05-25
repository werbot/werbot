// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v4.23.1
// source: info.proto

package proto

import (
	_ "github.com/envoyproxy/protoc-gen-validate/validate"
	proto "github.com/werbot/werbot/internal/grpc/user/proto"
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

// rpc UserMetrics
type UserMetrics struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UserMetrics) Reset() {
	*x = UserMetrics{}
	if protoimpl.UnsafeEnabled {
		mi := &file_info_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserMetrics) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserMetrics) ProtoMessage() {}

func (x *UserMetrics) ProtoReflect() protoreflect.Message {
	mi := &file_info_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserMetrics.ProtoReflect.Descriptor instead.
func (*UserMetrics) Descriptor() ([]byte, []int) {
	return file_info_proto_rawDescGZIP(), []int{0}
}

type UserMetrics_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	UserId string     `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty" query:"user_id" params:"user_id"`  
	Role   proto.Role `protobuf:"varint,2,opt,name=role,proto3,enum=user.Role" json:"role,omitempty"`
}

func (x *UserMetrics_Request) Reset() {
	*x = UserMetrics_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_info_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserMetrics_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserMetrics_Request) ProtoMessage() {}

func (x *UserMetrics_Request) ProtoReflect() protoreflect.Message {
	mi := &file_info_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserMetrics_Request.ProtoReflect.Descriptor instead.
func (*UserMetrics_Request) Descriptor() ([]byte, []int) {
	return file_info_proto_rawDescGZIP(), []int{0, 0}
}

func (x *UserMetrics_Request) GetUserId() string {
	if x != nil {
		return x.UserId
	}
	return ""
}

func (x *UserMetrics_Request) GetRole() proto.Role {
	if x != nil {
		return x.Role
	}
	return proto.Role(0)
}

type UserMetrics_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Users    int32 `protobuf:"varint,1,opt,name=users,proto3" json:"users,omitempty"`
	Projects int32 `protobuf:"varint,2,opt,name=projects,proto3" json:"projects,omitempty"`
	Servers  int32 `protobuf:"varint,3,opt,name=servers,proto3" json:"servers,omitempty"`
}

func (x *UserMetrics_Response) Reset() {
	*x = UserMetrics_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_info_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserMetrics_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserMetrics_Response) ProtoMessage() {}

func (x *UserMetrics_Response) ProtoReflect() protoreflect.Message {
	mi := &file_info_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserMetrics_Response.ProtoReflect.Descriptor instead.
func (*UserMetrics_Response) Descriptor() ([]byte, []int) {
	return file_info_proto_rawDescGZIP(), []int{0, 1}
}

func (x *UserMetrics_Response) GetUsers() int32 {
	if x != nil {
		return x.Users
	}
	return 0
}

func (x *UserMetrics_Response) GetProjects() int32 {
	if x != nil {
		return x.Projects
	}
	return 0
}

func (x *UserMetrics_Response) GetServers() int32 {
	if x != nil {
		return x.Servers
	}
	return 0
}

var File_info_proto protoreflect.FileDescriptor

var file_info_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x69, 0x6e, 0x66, 0x6f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x69, 0x6e,
	0x66, 0x6f, 0x1a, 0x17, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2f, 0x76, 0x61, 0x6c,
	0x69, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x23, 0x69, 0x6e, 0x74,
	0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x67, 0x72, 0x70, 0x63, 0x2f, 0x75, 0x73, 0x65, 0x72, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x75, 0x73, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0xb6, 0x01, 0x0a, 0x0b, 0x55, 0x73, 0x65, 0x72, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73,
	0x1a, 0x4f, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x24, 0x0a, 0x07, 0x75,
	0x73, 0x65, 0x72, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x0b, 0xfa, 0x42,
	0x08, 0x72, 0x06, 0xd0, 0x01, 0x01, 0xb0, 0x01, 0x01, 0x52, 0x06, 0x75, 0x73, 0x65, 0x72, 0x49,
	0x64, 0x12, 0x1e, 0x0a, 0x04, 0x72, 0x6f, 0x6c, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x0a, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x2e, 0x52, 0x6f, 0x6c, 0x65, 0x52, 0x04, 0x72, 0x6f, 0x6c,
	0x65, 0x1a, 0x56, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a,
	0x05, 0x75, 0x73, 0x65, 0x72, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x05, 0x75, 0x73,
	0x65, 0x72, 0x73, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x12,
	0x18, 0x0a, 0x07, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x07, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x73, 0x32, 0x56, 0x0a, 0x0c, 0x49, 0x6e, 0x66,
	0x6f, 0x48, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x72, 0x73, 0x12, 0x46, 0x0a, 0x0b, 0x55, 0x73, 0x65,
	0x72, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x12, 0x19, 0x2e, 0x69, 0x6e, 0x66, 0x6f, 0x2e,
	0x55, 0x73, 0x65, 0x72, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x69, 0x6e, 0x66, 0x6f, 0x2e, 0x55, 0x73, 0x65, 0x72, 0x4d,
	0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x42, 0x33, 0x5a, 0x31, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x69, 0x6e,
	0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x67, 0x72, 0x70, 0x63, 0x2f, 0x69, 0x6e, 0x66, 0x6f,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_info_proto_rawDescOnce sync.Once
	file_info_proto_rawDescData = file_info_proto_rawDesc
)

func file_info_proto_rawDescGZIP() []byte {
	file_info_proto_rawDescOnce.Do(func() {
		file_info_proto_rawDescData = protoimpl.X.CompressGZIP(file_info_proto_rawDescData)
	})
	return file_info_proto_rawDescData
}

var file_info_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_info_proto_goTypes = []interface{}{
	(*UserMetrics)(nil),          // 0: info.UserMetrics
	(*UserMetrics_Request)(nil),  // 1: info.UserMetrics.Request
	(*UserMetrics_Response)(nil), // 2: info.UserMetrics.Response
	(proto.Role)(0),              // 3: user.Role
}
var file_info_proto_depIdxs = []int32{
	3, // 0: info.UserMetrics.Request.role:type_name -> user.Role
	1, // 1: info.InfoHandlers.UserMetrics:input_type -> info.UserMetrics.Request
	2, // 2: info.InfoHandlers.UserMetrics:output_type -> info.UserMetrics.Response
	2, // [2:3] is the sub-list for method output_type
	1, // [1:2] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_info_proto_init() }
func file_info_proto_init() {
	if File_info_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_info_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserMetrics); i {
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
		file_info_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserMetrics_Request); i {
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
		file_info_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserMetrics_Response); i {
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
			RawDescriptor: file_info_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_info_proto_goTypes,
		DependencyIndexes: file_info_proto_depIdxs,
		MessageInfos:      file_info_proto_msgTypes,
	}.Build()
	File_info_proto = out.File
	file_info_proto_rawDesc = nil
	file_info_proto_goTypes = nil
	file_info_proto_depIdxs = nil
}
