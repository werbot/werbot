// Code generated by protoc-gen-go. DO NOT EDIT.
// source: firewall.proto

package firewall

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

type Rules int32

const (
	Rules_unspecified Rules = 0
	Rules_country     Rules = 1
	Rules_ip          Rules = 2
)

// Enum value maps for Rules.
var (
	Rules_name = map[int32]string{
		0: "unspecified",
		1: "country",
		2: "ip",
	}
	Rules_value = map[string]int32{
		"unspecified": 0,
		"country":     1,
		"ip":          2,
	}
)

func (x Rules) Enum() *Rules {
	p := new(Rules)
	*p = x
	return p
}

func (x Rules) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Rules) Descriptor() protoreflect.EnumDescriptor {
	return file_firewall_proto_enumTypes[0].Descriptor()
}

func (Rules) Type() protoreflect.EnumType {
	return &file_firewall_proto_enumTypes[0]
}

func (x Rules) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Rules.Descriptor instead.
func (Rules) EnumDescriptor() ([]byte, []int) {
	return file_firewall_proto_rawDescGZIP(), []int{0}
}

// -----------------------------------------------------
// global messages
type AccessPolicy struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Country int32 `protobuf:"varint,1,opt,name=country,proto3" json:"country,omitempty"`
	Network int32 `protobuf:"varint,2,opt,name=network,proto3" json:"network,omitempty"`
}

func (x *AccessPolicy) Reset() {
	*x = AccessPolicy{}
	mi := &file_firewall_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AccessPolicy) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AccessPolicy) ProtoMessage() {}

func (x *AccessPolicy) ProtoReflect() protoreflect.Message {
	mi := &file_firewall_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AccessPolicy.ProtoReflect.Descriptor instead.
func (*AccessPolicy) Descriptor() ([]byte, []int) {
	return file_firewall_proto_rawDescGZIP(), []int{0}
}

func (x *AccessPolicy) GetCountry() int32 {
	if x != nil {
		return x.Country
	}
	return 0
}

func (x *AccessPolicy) GetNetwork() int32 {
	if x != nil {
		return x.Network
	}
	return 0
}

type Country struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CountryId   string `protobuf:"bytes,1,opt,name=country_id,json=countryId,proto3" json:"country_id,omitempty"`
	SchemeId    string `protobuf:"bytes,2,opt,name=scheme_id,json=schemeId,proto3" json:"scheme_id,omitempty"`
	CountryName string `protobuf:"bytes,3,opt,name=country_name,json=countryName,proto3" json:"country_name,omitempty"`
	CountryCode string `protobuf:"bytes,4,opt,name=country_code,json=countryCode,proto3" json:"country_code,omitempty"`
}

func (x *Country) Reset() {
	*x = Country{}
	mi := &file_firewall_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Country) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Country) ProtoMessage() {}

func (x *Country) ProtoReflect() protoreflect.Message {
	mi := &file_firewall_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Country.ProtoReflect.Descriptor instead.
func (*Country) Descriptor() ([]byte, []int) {
	return file_firewall_proto_rawDescGZIP(), []int{1}
}

func (x *Country) GetCountryId() string {
	if x != nil {
		return x.CountryId
	}
	return ""
}

func (x *Country) GetSchemeId() string {
	if x != nil {
		return x.SchemeId
	}
	return ""
}

func (x *Country) GetCountryName() string {
	if x != nil {
		return x.CountryName
	}
	return ""
}

func (x *Country) GetCountryCode() string {
	if x != nil {
		return x.CountryCode
	}
	return ""
}

type Network struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NetworkId string `protobuf:"bytes,1,opt,name=network_id,json=networkId,proto3" json:"network_id,omitempty"`
	SchemeId  string `protobuf:"bytes,2,opt,name=scheme_id,json=schemeId,proto3" json:"scheme_id,omitempty"`
	Network   string `protobuf:"bytes,3,opt,name=network,proto3" json:"network,omitempty"`
}

func (x *Network) Reset() {
	*x = Network{}
	mi := &file_firewall_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Network) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Network) ProtoMessage() {}

func (x *Network) ProtoReflect() protoreflect.Message {
	mi := &file_firewall_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Network.ProtoReflect.Descriptor instead.
func (*Network) Descriptor() ([]byte, []int) {
	return file_firewall_proto_rawDescGZIP(), []int{2}
}

func (x *Network) GetNetworkId() string {
	if x != nil {
		return x.NetworkId
	}
	return ""
}

func (x *Network) GetSchemeId() string {
	if x != nil {
		return x.SchemeId
	}
	return ""
}

func (x *Network) GetNetwork() string {
	if x != nil {
		return x.Network
	}
	return ""
}

// rpc IPAccess
type IPAccess struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *IPAccess) Reset() {
	*x = IPAccess{}
	mi := &file_firewall_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *IPAccess) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IPAccess) ProtoMessage() {}

func (x *IPAccess) ProtoReflect() protoreflect.Message {
	mi := &file_firewall_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IPAccess.ProtoReflect.Descriptor instead.
func (*IPAccess) Descriptor() ([]byte, []int) {
	return file_firewall_proto_rawDescGZIP(), []int{3}
}

// rpc UpdateFirewallListData
type UpdateFirewallListData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UpdateFirewallListData) Reset() {
	*x = UpdateFirewallListData{}
	mi := &file_firewall_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *UpdateFirewallListData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateFirewallListData) ProtoMessage() {}

func (x *UpdateFirewallListData) ProtoReflect() protoreflect.Message {
	mi := &file_firewall_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateFirewallListData.ProtoReflect.Descriptor instead.
func (*UpdateFirewallListData) Descriptor() ([]byte, []int) {
	return file_firewall_proto_rawDescGZIP(), []int{4}
}

type IPAccess_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ClientIp string `protobuf:"bytes,1,opt,name=client_ip,json=clientIp,proto3" json:"-"`  
}

func (x *IPAccess_Request) Reset() {
	*x = IPAccess_Request{}
	mi := &file_firewall_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *IPAccess_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IPAccess_Request) ProtoMessage() {}

func (x *IPAccess_Request) ProtoReflect() protoreflect.Message {
	mi := &file_firewall_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IPAccess_Request.ProtoReflect.Descriptor instead.
func (*IPAccess_Request) Descriptor() ([]byte, []int) {
	return file_firewall_proto_rawDescGZIP(), []int{3, 0}
}

func (x *IPAccess_Request) GetClientIp() string {
	if x != nil {
		return x.ClientIp
	}
	return ""
}

type IPAccess_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CountryName string `protobuf:"bytes,1,opt,name=country_name,json=countryName,proto3" json:"country_name,omitempty"`
	CountryCode string `protobuf:"bytes,2,opt,name=country_code,json=countryCode,proto3" json:"country_code,omitempty"`
}

func (x *IPAccess_Response) Reset() {
	*x = IPAccess_Response{}
	mi := &file_firewall_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *IPAccess_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IPAccess_Response) ProtoMessage() {}

func (x *IPAccess_Response) ProtoReflect() protoreflect.Message {
	mi := &file_firewall_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IPAccess_Response.ProtoReflect.Descriptor instead.
func (*IPAccess_Response) Descriptor() ([]byte, []int) {
	return file_firewall_proto_rawDescGZIP(), []int{3, 1}
}

func (x *IPAccess_Response) GetCountryName() string {
	if x != nil {
		return x.CountryName
	}
	return ""
}

func (x *IPAccess_Response) GetCountryCode() string {
	if x != nil {
		return x.CountryCode
	}
	return ""
}

type UpdateFirewallListData_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UpdateFirewallListData_Request) Reset() {
	*x = UpdateFirewallListData_Request{}
	mi := &file_firewall_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *UpdateFirewallListData_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateFirewallListData_Request) ProtoMessage() {}

func (x *UpdateFirewallListData_Request) ProtoReflect() protoreflect.Message {
	mi := &file_firewall_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateFirewallListData_Request.ProtoReflect.Descriptor instead.
func (*UpdateFirewallListData_Request) Descriptor() ([]byte, []int) {
	return file_firewall_proto_rawDescGZIP(), []int{4, 0}
}

type UpdateFirewallListData_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UpdateFirewallListData_Response) Reset() {
	*x = UpdateFirewallListData_Response{}
	mi := &file_firewall_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *UpdateFirewallListData_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateFirewallListData_Response) ProtoMessage() {}

func (x *UpdateFirewallListData_Response) ProtoReflect() protoreflect.Message {
	mi := &file_firewall_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateFirewallListData_Response.ProtoReflect.Descriptor instead.
func (*UpdateFirewallListData_Response) Descriptor() ([]byte, []int) {
	return file_firewall_proto_rawDescGZIP(), []int{4, 1}
}

var File_firewall_proto protoreflect.FileDescriptor

var file_firewall_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x66, 0x69, 0x72, 0x65, 0x77, 0x61, 0x6c, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x08, 0x66, 0x69, 0x72, 0x65, 0x77, 0x61, 0x6c, 0x6c, 0x1a, 0x1b, 0x62, 0x75, 0x66, 0x2f,
	0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x5e, 0x0a, 0x0c, 0x41, 0x63, 0x63, 0x65, 0x73,
	0x73, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x12, 0x26, 0x0a, 0x07, 0x63, 0x6f, 0x75, 0x6e, 0x74,
	0x72, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x42, 0x0c, 0xba, 0x48, 0x09, 0xc8, 0x01, 0x01,
	0x1a, 0x04, 0x30, 0x00, 0x30, 0x01, 0x52, 0x07, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x12,
	0x26, 0x0a, 0x07, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05,
	0x42, 0x0c, 0xba, 0x48, 0x09, 0xc8, 0x01, 0x01, 0x1a, 0x04, 0x30, 0x00, 0x30, 0x01, 0x52, 0x07,
	0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x22, 0xa9, 0x01, 0x0a, 0x07, 0x43, 0x6f, 0x75, 0x6e,
	0x74, 0x72, 0x79, 0x12, 0x27, 0x0a, 0x0a, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x5f, 0x69,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x08, 0xba, 0x48, 0x05, 0x72, 0x03, 0xb0, 0x01,
	0x01, 0x52, 0x09, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x49, 0x64, 0x12, 0x25, 0x0a, 0x09,
	0x73, 0x63, 0x68, 0x65, 0x6d, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42,
	0x08, 0xba, 0x48, 0x05, 0x72, 0x03, 0xb0, 0x01, 0x01, 0x52, 0x08, 0x73, 0x63, 0x68, 0x65, 0x6d,
	0x65, 0x49, 0x64, 0x12, 0x21, 0x0a, 0x0c, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x5f, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x63, 0x6f, 0x75, 0x6e, 0x74,
	0x72, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x2b, 0x0a, 0x0c, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72,
	0x79, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x42, 0x08, 0xba, 0x48,
	0x05, 0x72, 0x03, 0x98, 0x01, 0x02, 0x52, 0x0b, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x43,
	0x6f, 0x64, 0x65, 0x22, 0x7c, 0x0a, 0x07, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x12, 0x27,
	0x0a, 0x0a, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x42, 0x08, 0xba, 0x48, 0x05, 0x72, 0x03, 0xb0, 0x01, 0x01, 0x52, 0x09, 0x6e, 0x65,
	0x74, 0x77, 0x6f, 0x72, 0x6b, 0x49, 0x64, 0x12, 0x25, 0x0a, 0x09, 0x73, 0x63, 0x68, 0x65, 0x6d,
	0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x08, 0xba, 0x48, 0x05, 0x72,
	0x03, 0xb0, 0x01, 0x01, 0x52, 0x08, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x65, 0x49, 0x64, 0x12, 0x21,
	0x0a, 0x07, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x42,
	0x07, 0xba, 0x48, 0x04, 0x72, 0x02, 0x70, 0x01, 0x52, 0x07, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72,
	0x6b, 0x22, 0x90, 0x01, 0x0a, 0x08, 0x49, 0x50, 0x41, 0x63, 0x63, 0x65, 0x73, 0x73, 0x1a, 0x32,
	0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x27, 0x0a, 0x09, 0x63, 0x6c, 0x69,
	0x65, 0x6e, 0x74, 0x5f, 0x69, 0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x0a, 0xba, 0x48,
	0x07, 0xc8, 0x01, 0x01, 0x72, 0x02, 0x70, 0x01, 0x52, 0x08, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74,
	0x49, 0x70, 0x1a, 0x50, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x21,
	0x0a, 0x0c, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x4e, 0x61, 0x6d,
	0x65, 0x12, 0x21, 0x0a, 0x0c, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x5f, 0x63, 0x6f, 0x64,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79,
	0x43, 0x6f, 0x64, 0x65, 0x22, 0x2f, 0x0a, 0x16, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x46, 0x69,
	0x72, 0x65, 0x77, 0x61, 0x6c, 0x6c, 0x4c, 0x69, 0x73, 0x74, 0x44, 0x61, 0x74, 0x61, 0x1a, 0x09,
	0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x0a, 0x08, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x2a, 0x2d, 0x0a, 0x05, 0x52, 0x75, 0x6c, 0x65, 0x73, 0x12, 0x0f,
	0x0a, 0x0b, 0x75, 0x6e, 0x73, 0x70, 0x65, 0x63, 0x69, 0x66, 0x69, 0x65, 0x64, 0x10, 0x00, 0x12,
	0x0b, 0x0a, 0x07, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x10, 0x01, 0x12, 0x06, 0x0a, 0x02,
	0x69, 0x70, 0x10, 0x02, 0x32, 0xca, 0x01, 0x0a, 0x10, 0x46, 0x69, 0x72, 0x65, 0x77, 0x61, 0x6c,
	0x6c, 0x48, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x72, 0x73, 0x12, 0x45, 0x0a, 0x08, 0x49, 0x50, 0x41,
	0x63, 0x63, 0x65, 0x73, 0x73, 0x12, 0x1a, 0x2e, 0x66, 0x69, 0x72, 0x65, 0x77, 0x61, 0x6c, 0x6c,
	0x2e, 0x49, 0x50, 0x41, 0x63, 0x63, 0x65, 0x73, 0x73, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x1b, 0x2e, 0x66, 0x69, 0x72, 0x65, 0x77, 0x61, 0x6c, 0x6c, 0x2e, 0x49, 0x50, 0x41,
	0x63, 0x63, 0x65, 0x73, 0x73, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x12, 0x6f, 0x0a, 0x16, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x46, 0x69, 0x72, 0x65, 0x77, 0x61,
	0x6c, 0x6c, 0x4c, 0x69, 0x73, 0x74, 0x44, 0x61, 0x74, 0x61, 0x12, 0x28, 0x2e, 0x66, 0x69, 0x72,
	0x65, 0x77, 0x61, 0x6c, 0x6c, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x46, 0x69, 0x72, 0x65,
	0x77, 0x61, 0x6c, 0x6c, 0x4c, 0x69, 0x73, 0x74, 0x44, 0x61, 0x74, 0x61, 0x2e, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x29, 0x2e, 0x66, 0x69, 0x72, 0x65, 0x77, 0x61, 0x6c, 0x6c, 0x2e,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x46, 0x69, 0x72, 0x65, 0x77, 0x61, 0x6c, 0x6c, 0x4c, 0x69,
	0x73, 0x74, 0x44, 0x61, 0x74, 0x61, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x42, 0x40, 0x5a, 0x3e, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x69, 0x6e,
	0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x63, 0x6f, 0x72, 0x65, 0x2f, 0x66, 0x69, 0x72, 0x65,
	0x77, 0x61, 0x6c, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x66, 0x69, 0x72, 0x65, 0x77,
	0x61, 0x6c, 0x6c, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_firewall_proto_rawDescOnce sync.Once
	file_firewall_proto_rawDescData = file_firewall_proto_rawDesc
)

func file_firewall_proto_rawDescGZIP() []byte {
	file_firewall_proto_rawDescOnce.Do(func() {
		file_firewall_proto_rawDescData = protoimpl.X.CompressGZIP(file_firewall_proto_rawDescData)
	})
	return file_firewall_proto_rawDescData
}

var file_firewall_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_firewall_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_firewall_proto_goTypes = []any{
	(Rules)(0),                              // 0: firewall.Rules
	(*AccessPolicy)(nil),                    // 1: firewall.AccessPolicy
	(*Country)(nil),                         // 2: firewall.Country
	(*Network)(nil),                         // 3: firewall.Network
	(*IPAccess)(nil),                        // 4: firewall.IPAccess
	(*UpdateFirewallListData)(nil),          // 5: firewall.UpdateFirewallListData
	(*IPAccess_Request)(nil),                // 6: firewall.IPAccess.Request
	(*IPAccess_Response)(nil),               // 7: firewall.IPAccess.Response
	(*UpdateFirewallListData_Request)(nil),  // 8: firewall.UpdateFirewallListData.Request
	(*UpdateFirewallListData_Response)(nil), // 9: firewall.UpdateFirewallListData.Response
}
var file_firewall_proto_depIdxs = []int32{
	6, // 0: firewall.FirewallHandlers.IPAccess:input_type -> firewall.IPAccess.Request
	8, // 1: firewall.FirewallHandlers.UpdateFirewallListData:input_type -> firewall.UpdateFirewallListData.Request
	7, // 2: firewall.FirewallHandlers.IPAccess:output_type -> firewall.IPAccess.Response
	9, // 3: firewall.FirewallHandlers.UpdateFirewallListData:output_type -> firewall.UpdateFirewallListData.Response
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_firewall_proto_init() }
func file_firewall_proto_init() {
	if File_firewall_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_firewall_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_firewall_proto_goTypes,
		DependencyIndexes: file_firewall_proto_depIdxs,
		EnumInfos:         file_firewall_proto_enumTypes,
		MessageInfos:      file_firewall_proto_msgTypes,
	}.Build()
	File_firewall_proto = out.File
	file_firewall_proto_rawDesc = nil
	file_firewall_proto_goTypes = nil
	file_firewall_proto_depIdxs = nil
}
