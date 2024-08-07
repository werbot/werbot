// Code generated by protoc-gen-go. DO NOT EDIT.
// source: utility.proto

package proto

import (
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
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

// rpc GetInfo
type Countries struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Countries) Reset() {
	*x = Countries{}
	if protoimpl.UnsafeEnabled {
		mi := &file_utility_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Countries) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Countries) ProtoMessage() {}

func (x *Countries) ProtoReflect() protoreflect.Message {
	mi := &file_utility_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Countries.ProtoReflect.Descriptor instead.
func (*Countries) Descriptor() ([]byte, []int) {
	return file_utility_proto_rawDescGZIP(), []int{0}
}

// rpc CountryByIP
type CountryByIP struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *CountryByIP) Reset() {
	*x = CountryByIP{}
	if protoimpl.UnsafeEnabled {
		mi := &file_utility_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CountryByIP) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CountryByIP) ProtoMessage() {}

func (x *CountryByIP) ProtoReflect() protoreflect.Message {
	mi := &file_utility_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CountryByIP.ProtoReflect.Descriptor instead.
func (*CountryByIP) Descriptor() ([]byte, []int) {
	return file_utility_proto_rawDescGZIP(), []int{1}
}

type Countries_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Countries_Request) Reset() {
	*x = Countries_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_utility_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Countries_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Countries_Request) ProtoMessage() {}

func (x *Countries_Request) ProtoReflect() protoreflect.Message {
	mi := &file_utility_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Countries_Request.ProtoReflect.Descriptor instead.
func (*Countries_Request) Descriptor() ([]byte, []int) {
	return file_utility_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Countries_Request) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type Countries_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Countries []*Countries_Country `protobuf:"bytes,1,rep,name=countries,proto3" json:"countries,omitempty"`
}

func (x *Countries_Response) Reset() {
	*x = Countries_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_utility_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Countries_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Countries_Response) ProtoMessage() {}

func (x *Countries_Response) ProtoReflect() protoreflect.Message {
	mi := &file_utility_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Countries_Response.ProtoReflect.Descriptor instead.
func (*Countries_Response) Descriptor() ([]byte, []int) {
	return file_utility_proto_rawDescGZIP(), []int{0, 1}
}

func (x *Countries_Response) GetCountries() []*Countries_Country {
	if x != nil {
		return x.Countries
	}
	return nil
}

type Countries_Country struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code string `protobuf:"bytes,1,opt,name=code,proto3" json:"code,omitempty"`
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Countries_Country) Reset() {
	*x = Countries_Country{}
	if protoimpl.UnsafeEnabled {
		mi := &file_utility_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Countries_Country) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Countries_Country) ProtoMessage() {}

func (x *Countries_Country) ProtoReflect() protoreflect.Message {
	mi := &file_utility_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Countries_Country.ProtoReflect.Descriptor instead.
func (*Countries_Country) Descriptor() ([]byte, []int) {
	return file_utility_proto_rawDescGZIP(), []int{0, 2}
}

func (x *Countries_Country) GetCode() string {
	if x != nil {
		return x.Code
	}
	return ""
}

func (x *Countries_Country) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type CountryByIP_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ip string `protobuf:"bytes,1,opt,name=ip,proto3" json:"ip,omitempty"`
}

func (x *CountryByIP_Request) Reset() {
	*x = CountryByIP_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_utility_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CountryByIP_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CountryByIP_Request) ProtoMessage() {}

func (x *CountryByIP_Request) ProtoReflect() protoreflect.Message {
	mi := &file_utility_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CountryByIP_Request.ProtoReflect.Descriptor instead.
func (*CountryByIP_Request) Descriptor() ([]byte, []int) {
	return file_utility_proto_rawDescGZIP(), []int{1, 0}
}

func (x *CountryByIP_Request) GetIp() string {
	if x != nil {
		return x.Ip
	}
	return ""
}

type CountryByIP_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Code string `protobuf:"bytes,2,opt,name=code,proto3" json:"code,omitempty"`
}

func (x *CountryByIP_Response) Reset() {
	*x = CountryByIP_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_utility_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CountryByIP_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CountryByIP_Response) ProtoMessage() {}

func (x *CountryByIP_Response) ProtoReflect() protoreflect.Message {
	mi := &file_utility_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CountryByIP_Response.ProtoReflect.Descriptor instead.
func (*CountryByIP_Response) Descriptor() ([]byte, []int) {
	return file_utility_proto_rawDescGZIP(), []int{1, 1}
}

func (x *CountryByIP_Response) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *CountryByIP_Response) GetCode() string {
	if x != nil {
		return x.Code
	}
	return ""
}

var File_utility_proto protoreflect.FileDescriptor

var file_utility_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x75, 0x74, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x07, 0x75, 0x74, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x1a, 0x1b, 0x62, 0x75, 0x66, 0x2f, 0x76, 0x61,
	0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xac, 0x01, 0x0a, 0x09, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72,
	0x69, 0x65, 0x73, 0x1a, 0x26, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1b,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x07, 0xba, 0x48,
	0x04, 0x72, 0x02, 0x10, 0x02, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x1a, 0x44, 0x0a, 0x08, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x38, 0x0a, 0x09, 0x63, 0x6f, 0x75, 0x6e, 0x74,
	0x72, 0x69, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x75, 0x74, 0x69,
	0x6c, 0x69, 0x74, 0x79, 0x2e, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x69, 0x65, 0x73, 0x2e, 0x43,
	0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x09, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x69, 0x65,
	0x73, 0x1a, 0x31, 0x0a, 0x07, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x12, 0x0a, 0x04,
	0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x22, 0x65, 0x0a, 0x0b, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x42,
	0x79, 0x49, 0x50, 0x1a, 0x22, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x17,
	0x0a, 0x02, 0x69, 0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x07, 0xba, 0x48, 0x04, 0x72,
	0x02, 0x70, 0x01, 0x52, 0x02, 0x69, 0x70, 0x1a, 0x32, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x32, 0xa7, 0x01, 0x0a, 0x0f,
	0x55, 0x74, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x48, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x72, 0x73, 0x12,
	0x46, 0x0a, 0x09, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x69, 0x65, 0x73, 0x12, 0x1a, 0x2e, 0x75,
	0x74, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x2e, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x69, 0x65, 0x73,
	0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1b, 0x2e, 0x75, 0x74, 0x69, 0x6c, 0x69,
	0x74, 0x79, 0x2e, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x69, 0x65, 0x73, 0x2e, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x4c, 0x0a, 0x0b, 0x43, 0x6f, 0x75, 0x6e, 0x74,
	0x72, 0x79, 0x42, 0x79, 0x49, 0x50, 0x12, 0x1c, 0x2e, 0x75, 0x74, 0x69, 0x6c, 0x69, 0x74, 0x79,
	0x2e, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x42, 0x79, 0x49, 0x50, 0x2e, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x1d, 0x2e, 0x75, 0x74, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x2e, 0x43,
	0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x42, 0x79, 0x49, 0x50, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x36, 0x5a, 0x34, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x77, 0x65, 0x72, 0x62, 0x6f,
	0x74, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x67, 0x72, 0x70, 0x63, 0x2f,
	0x75, 0x74, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_utility_proto_rawDescOnce sync.Once
	file_utility_proto_rawDescData = file_utility_proto_rawDesc
)

func file_utility_proto_rawDescGZIP() []byte {
	file_utility_proto_rawDescOnce.Do(func() {
		file_utility_proto_rawDescData = protoimpl.X.CompressGZIP(file_utility_proto_rawDescData)
	})
	return file_utility_proto_rawDescData
}

var file_utility_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_utility_proto_goTypes = []any{
	(*Countries)(nil),            // 0: utility.Countries
	(*CountryByIP)(nil),          // 1: utility.CountryByIP
	(*Countries_Request)(nil),    // 2: utility.Countries.Request
	(*Countries_Response)(nil),   // 3: utility.Countries.Response
	(*Countries_Country)(nil),    // 4: utility.Countries.Country
	(*CountryByIP_Request)(nil),  // 5: utility.CountryByIP.Request
	(*CountryByIP_Response)(nil), // 6: utility.CountryByIP.Response
}
var file_utility_proto_depIdxs = []int32{
	4, // 0: utility.Countries.Response.countries:type_name -> utility.Countries.Country
	2, // 1: utility.UtilityHandlers.Countries:input_type -> utility.Countries.Request
	5, // 2: utility.UtilityHandlers.CountryByIP:input_type -> utility.CountryByIP.Request
	3, // 3: utility.UtilityHandlers.Countries:output_type -> utility.Countries.Response
	6, // 4: utility.UtilityHandlers.CountryByIP:output_type -> utility.CountryByIP.Response
	3, // [3:5] is the sub-list for method output_type
	1, // [1:3] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_utility_proto_init() }
func file_utility_proto_init() {
	if File_utility_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_utility_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Countries); i {
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
		file_utility_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*CountryByIP); i {
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
		file_utility_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*Countries_Request); i {
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
		file_utility_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*Countries_Response); i {
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
		file_utility_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*Countries_Country); i {
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
		file_utility_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*CountryByIP_Request); i {
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
		file_utility_proto_msgTypes[6].Exporter = func(v any, i int) any {
			switch v := v.(*CountryByIP_Response); i {
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
			RawDescriptor: file_utility_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_utility_proto_goTypes,
		DependencyIndexes: file_utility_proto_depIdxs,
		MessageInfos:      file_utility_proto_msgTypes,
	}.Build()
	File_utility_proto = out.File
	file_utility_proto_rawDesc = nil
	file_utility_proto_goTypes = nil
	file_utility_proto_depIdxs = nil
}
