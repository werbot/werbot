// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.7
// source: billing.proto

package billing

import (
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

// -----------------------------------------------------
// global messages
type PackageDimensions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Height float32 `protobuf:"fixed32,1,opt,name=height,proto3" json:"height,omitempty"`
	Length float32 `protobuf:"fixed32,2,opt,name=length,proto3" json:"length,omitempty"`
	Weight float32 `protobuf:"fixed32,3,opt,name=weight,proto3" json:"weight,omitempty"`
	Width  float32 `protobuf:"fixed32,4,opt,name=width,proto3" json:"width,omitempty"`
}

func (x *PackageDimensions) Reset() {
	*x = PackageDimensions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_billing_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PackageDimensions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PackageDimensions) ProtoMessage() {}

func (x *PackageDimensions) ProtoReflect() protoreflect.Message {
	mi := &file_billing_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PackageDimensions.ProtoReflect.Descriptor instead.
func (*PackageDimensions) Descriptor() ([]byte, []int) {
	return file_billing_proto_rawDescGZIP(), []int{0}
}

func (x *PackageDimensions) GetHeight() float32 {
	if x != nil {
		return x.Height
	}
	return 0
}

func (x *PackageDimensions) GetLength() float32 {
	if x != nil {
		return x.Length
	}
	return 0
}

func (x *PackageDimensions) GetWeight() float32 {
	if x != nil {
		return x.Weight
	}
	return 0
}

func (x *PackageDimensions) GetWidth() float32 {
	if x != nil {
		return x.Width
	}
	return 0
}

// rpc UpdateProduct
type UpdateProduct struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UpdateProduct) Reset() {
	*x = UpdateProduct{}
	if protoimpl.UnsafeEnabled {
		mi := &file_billing_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateProduct) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateProduct) ProtoMessage() {}

func (x *UpdateProduct) ProtoReflect() protoreflect.Message {
	mi := &file_billing_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateProduct.ProtoReflect.Descriptor instead.
func (*UpdateProduct) Descriptor() ([]byte, []int) {
	return file_billing_proto_rawDescGZIP(), []int{1}
}

type UpdateProduct_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PlanName            bool               `protobuf:"varint,1,opt,name=plan_name,json=planName,proto3" json:"plan_name,omitempty"`
	Attributes          []string           `protobuf:"bytes,2,rep,name=attributes,proto3" json:"attributes,omitempty"`
	Caption             string             `protobuf:"bytes,3,opt,name=caption,proto3" json:"caption,omitempty"`
	DeactivateOn        string             `protobuf:"bytes,4,opt,name=deactivate_on,json=deactivateOn,proto3" json:"deactivate_on,omitempty"`
	Description         string             `protobuf:"bytes,5,opt,name=description,proto3" json:"description,omitempty"`
	Id                  string             `protobuf:"bytes,6,opt,name=id,proto3" json:"id,omitempty"`
	Images              []string           `protobuf:"bytes,7,rep,name=images,proto3" json:"images,omitempty"`
	Name                string             `protobuf:"bytes,8,opt,name=name,proto3" json:"name,omitempty"`
	PackageDimensions   *PackageDimensions `protobuf:"bytes,9,opt,name=package_dimensions,json=packageDimensions,proto3" json:"package_dimensions,omitempty"`
	Shippable           bool               `protobuf:"varint,11,opt,name=shippable,proto3" json:"shippable,omitempty"`
	StatementDescriptor string             `protobuf:"bytes,12,opt,name=statement_descriptor,json=statementDescriptor,proto3" json:"statement_descriptor,omitempty"`
	TaxCode             string             `protobuf:"bytes,13,opt,name=tax_code,json=taxCode,proto3" json:"tax_code,omitempty"`
	Type                string             `protobuf:"bytes,14,opt,name=type,proto3" json:"type,omitempty"`
	UnitLabel           string             `protobuf:"bytes,15,opt,name=unit_label,json=unitLabel,proto3" json:"unit_label,omitempty"`
	Url                 string             `protobuf:"bytes,16,opt,name=url,proto3" json:"url,omitempty"`
}

func (x *UpdateProduct_Request) Reset() {
	*x = UpdateProduct_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_billing_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateProduct_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateProduct_Request) ProtoMessage() {}

func (x *UpdateProduct_Request) ProtoReflect() protoreflect.Message {
	mi := &file_billing_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateProduct_Request.ProtoReflect.Descriptor instead.
func (*UpdateProduct_Request) Descriptor() ([]byte, []int) {
	return file_billing_proto_rawDescGZIP(), []int{1, 0}
}

func (x *UpdateProduct_Request) GetPlanName() bool {
	if x != nil {
		return x.PlanName
	}
	return false
}

func (x *UpdateProduct_Request) GetAttributes() []string {
	if x != nil {
		return x.Attributes
	}
	return nil
}

func (x *UpdateProduct_Request) GetCaption() string {
	if x != nil {
		return x.Caption
	}
	return ""
}

func (x *UpdateProduct_Request) GetDeactivateOn() string {
	if x != nil {
		return x.DeactivateOn
	}
	return ""
}

func (x *UpdateProduct_Request) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *UpdateProduct_Request) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *UpdateProduct_Request) GetImages() []string {
	if x != nil {
		return x.Images
	}
	return nil
}

func (x *UpdateProduct_Request) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *UpdateProduct_Request) GetPackageDimensions() *PackageDimensions {
	if x != nil {
		return x.PackageDimensions
	}
	return nil
}

func (x *UpdateProduct_Request) GetShippable() bool {
	if x != nil {
		return x.Shippable
	}
	return false
}

func (x *UpdateProduct_Request) GetStatementDescriptor() string {
	if x != nil {
		return x.StatementDescriptor
	}
	return ""
}

func (x *UpdateProduct_Request) GetTaxCode() string {
	if x != nil {
		return x.TaxCode
	}
	return ""
}

func (x *UpdateProduct_Request) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *UpdateProduct_Request) GetUnitLabel() string {
	if x != nil {
		return x.UnitLabel
	}
	return ""
}

func (x *UpdateProduct_Request) GetUrl() string {
	if x != nil {
		return x.Url
	}
	return ""
}

type UpdateProduct_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UpdateProduct_Response) Reset() {
	*x = UpdateProduct_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_billing_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateProduct_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateProduct_Response) ProtoMessage() {}

func (x *UpdateProduct_Response) ProtoReflect() protoreflect.Message {
	mi := &file_billing_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateProduct_Response.ProtoReflect.Descriptor instead.
func (*UpdateProduct_Response) Descriptor() ([]byte, []int) {
	return file_billing_proto_rawDescGZIP(), []int{1, 1}
}

var File_billing_proto protoreflect.FileDescriptor

var file_billing_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x07, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x22, 0x71, 0x0a, 0x11, 0x50, 0x61, 0x63, 0x6b,
	0x61, 0x67, 0x65, 0x44, 0x69, 0x6d, 0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x16, 0x0a,
	0x06, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x02, 0x52, 0x06, 0x68,
	0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x6c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x02, 0x52, 0x06, 0x6c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x12, 0x16, 0x0a,
	0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x02, 0x52, 0x06, 0x77,
	0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x77, 0x69, 0x64, 0x74, 0x68, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x02, 0x52, 0x05, 0x77, 0x69, 0x64, 0x74, 0x68, 0x22, 0xfd, 0x03, 0x0a, 0x0d,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x1a, 0xdf, 0x03,
	0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x70, 0x6c, 0x61,
	0x6e, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x70, 0x6c,
	0x61, 0x6e, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x61, 0x74, 0x74, 0x72, 0x69, 0x62,
	0x75, 0x74, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0a, 0x61, 0x74, 0x74, 0x72,
	0x69, 0x62, 0x75, 0x74, 0x65, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x61, 0x70, 0x74, 0x69, 0x6f,
	0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x61, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x23, 0x0a, 0x0d, 0x64, 0x65, 0x61, 0x63, 0x74, 0x69, 0x76, 0x61, 0x74, 0x65, 0x5f, 0x6f,
	0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x64, 0x65, 0x61, 0x63, 0x74, 0x69, 0x76,
	0x61, 0x74, 0x65, 0x4f, 0x6e, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63,
	0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x16, 0x0a, 0x06, 0x69, 0x6d, 0x61, 0x67, 0x65,
	0x73, 0x18, 0x07, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x69, 0x6d, 0x61, 0x67, 0x65, 0x73, 0x12,
	0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x12, 0x49, 0x0a, 0x12, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x64,
	0x69, 0x6d, 0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x1a, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67,
	0x65, 0x44, 0x69, 0x6d, 0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x11, 0x70, 0x61, 0x63,
	0x6b, 0x61, 0x67, 0x65, 0x44, 0x69, 0x6d, 0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x1c,
	0x0a, 0x09, 0x73, 0x68, 0x69, 0x70, 0x70, 0x61, 0x62, 0x6c, 0x65, 0x18, 0x0b, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x09, 0x73, 0x68, 0x69, 0x70, 0x70, 0x61, 0x62, 0x6c, 0x65, 0x12, 0x31, 0x0a, 0x14,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x5f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x6f, 0x72, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x09, 0x52, 0x13, 0x73, 0x74, 0x61, 0x74,
	0x65, 0x6d, 0x65, 0x6e, 0x74, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x12,
	0x19, 0x0a, 0x08, 0x74, 0x61, 0x78, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x0d, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x07, 0x74, 0x61, 0x78, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79,
	0x70, 0x65, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x1d,
	0x0a, 0x0a, 0x75, 0x6e, 0x69, 0x74, 0x5f, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x18, 0x0f, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x09, 0x75, 0x6e, 0x69, 0x74, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x12, 0x10, 0x0a,
	0x03, 0x75, 0x72, 0x6c, 0x18, 0x10, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x6c, 0x1a,
	0x0a, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0x65, 0x0a, 0x0f, 0x42,
	0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x48, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x72, 0x73, 0x12, 0x52,
	0x0a, 0x0d, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x12,
	0x1e, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65,
	0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x1f, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65,
	0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x22, 0x00, 0x42, 0x2c, 0x5a, 0x2a, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x61,
	0x70, 0x69, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_billing_proto_rawDescOnce sync.Once
	file_billing_proto_rawDescData = file_billing_proto_rawDesc
)

func file_billing_proto_rawDescGZIP() []byte {
	file_billing_proto_rawDescOnce.Do(func() {
		file_billing_proto_rawDescData = protoimpl.X.CompressGZIP(file_billing_proto_rawDescData)
	})
	return file_billing_proto_rawDescData
}

var file_billing_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_billing_proto_goTypes = []interface{}{
	(*PackageDimensions)(nil),      // 0: billing.PackageDimensions
	(*UpdateProduct)(nil),          // 1: billing.UpdateProduct
	(*UpdateProduct_Request)(nil),  // 2: billing.UpdateProduct.Request
	(*UpdateProduct_Response)(nil), // 3: billing.UpdateProduct.Response
}
var file_billing_proto_depIdxs = []int32{
	0, // 0: billing.UpdateProduct.Request.package_dimensions:type_name -> billing.PackageDimensions
	2, // 1: billing.BillingHandlers.UpdateProduct:input_type -> billing.UpdateProduct.Request
	3, // 2: billing.BillingHandlers.UpdateProduct:output_type -> billing.UpdateProduct.Response
	2, // [2:3] is the sub-list for method output_type
	1, // [1:2] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_billing_proto_init() }
func file_billing_proto_init() {
	if File_billing_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_billing_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PackageDimensions); i {
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
		file_billing_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateProduct); i {
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
		file_billing_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateProduct_Request); i {
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
		file_billing_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateProduct_Response); i {
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
			RawDescriptor: file_billing_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_billing_proto_goTypes,
		DependencyIndexes: file_billing_proto_depIdxs,
		MessageInfos:      file_billing_proto_msgTypes,
	}.Build()
	File_billing_proto = out.File
	file_billing_proto_rawDesc = nil
	file_billing_proto_goTypes = nil
	file_billing_proto_depIdxs = nil
}
