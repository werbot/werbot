// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.7
// source: billing.proto

package billing

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

// rpc Product
type Product struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Product) Reset() {
	*x = Product{}
	if protoimpl.UnsafeEnabled {
		mi := &file_billing_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Product) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Product) ProtoMessage() {}

func (x *Product) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use Product.ProtoReflect.Descriptor instead.
func (*Product) Descriptor() ([]byte, []int) {
	return file_billing_proto_rawDescGZIP(), []int{1}
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
		mi := &file_billing_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateProduct) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateProduct) ProtoMessage() {}

func (x *UpdateProduct) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use UpdateProduct.ProtoReflect.Descriptor instead.
func (*UpdateProduct) Descriptor() ([]byte, []int) {
	return file_billing_proto_rawDescGZIP(), []int{2}
}

type Product_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ProductId string `protobuf:"bytes,1,opt,name=product_id,json=productId,proto3" json:"product_id,omitempty"`
}

func (x *Product_Request) Reset() {
	*x = Product_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_billing_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Product_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Product_Request) ProtoMessage() {}

func (x *Product_Request) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use Product_Request.ProtoReflect.Descriptor instead.
func (*Product_Request) Descriptor() ([]byte, []int) {
	return file_billing_proto_rawDescGZIP(), []int{1, 0}
}

func (x *Product_Request) GetProductId() string {
	if x != nil {
		return x.ProductId
	}
	return ""
}

type Product_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Active              bool               `protobuf:"varint,1,opt,name=active,proto3" json:"active,omitempty"`
	Attributes          []string           `protobuf:"bytes,2,rep,name=attributes,proto3" json:"attributes,omitempty"`
	Caption             string             `protobuf:"bytes,3,opt,name=caption,proto3" json:"caption,omitempty"`
	Created             int32              `protobuf:"varint,4,opt,name=created,proto3" json:"created,omitempty"`
	DeactivateOn        string             `protobuf:"bytes,5,opt,name=deactivate_on,json=deactivateOn,proto3" json:"deactivate_on,omitempty"`
	Deleted             bool               `protobuf:"varint,6,opt,name=deleted,proto3" json:"deleted,omitempty"`
	Description         string             `protobuf:"bytes,7,opt,name=description,proto3" json:"description,omitempty"`
	Id                  string             `protobuf:"bytes,8,opt,name=id,proto3" json:"id,omitempty"`
	Images              []string           `protobuf:"bytes,9,rep,name=images,proto3" json:"images,omitempty"`
	Livemode            bool               `protobuf:"varint,10,opt,name=livemode,proto3" json:"livemode,omitempty"`
	Metadata            map[string]string  `protobuf:"bytes,11,rep,name=metadata,proto3" json:"metadata,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Name                string             `protobuf:"bytes,12,opt,name=name,proto3" json:"name,omitempty"`
	Object              string             `protobuf:"bytes,13,opt,name=object,proto3" json:"object,omitempty"`
	PackageDimensions   *PackageDimensions `protobuf:"bytes,14,opt,name=package_dimensions,json=packageDimensions,proto3" json:"package_dimensions,omitempty"`
	Shippable           bool               `protobuf:"varint,15,opt,name=shippable,proto3" json:"shippable,omitempty"`
	StatementDescriptor string             `protobuf:"bytes,16,opt,name=statement_descriptor,json=statementDescriptor,proto3" json:"statement_descriptor,omitempty"`
	Type                string             `protobuf:"bytes,17,opt,name=type,proto3" json:"type,omitempty"`
	UnitLabel           string             `protobuf:"bytes,18,opt,name=unit_label,json=unitLabel,proto3" json:"unit_label,omitempty"`
	Updated             int32              `protobuf:"varint,19,opt,name=updated,proto3" json:"updated,omitempty"`
	Url                 string             `protobuf:"bytes,20,opt,name=url,proto3" json:"url,omitempty"`
}

func (x *Product_Response) Reset() {
	*x = Product_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_billing_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Product_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Product_Response) ProtoMessage() {}

func (x *Product_Response) ProtoReflect() protoreflect.Message {
	mi := &file_billing_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Product_Response.ProtoReflect.Descriptor instead.
func (*Product_Response) Descriptor() ([]byte, []int) {
	return file_billing_proto_rawDescGZIP(), []int{1, 1}
}

func (x *Product_Response) GetActive() bool {
	if x != nil {
		return x.Active
	}
	return false
}

func (x *Product_Response) GetAttributes() []string {
	if x != nil {
		return x.Attributes
	}
	return nil
}

func (x *Product_Response) GetCaption() string {
	if x != nil {
		return x.Caption
	}
	return ""
}

func (x *Product_Response) GetCreated() int32 {
	if x != nil {
		return x.Created
	}
	return 0
}

func (x *Product_Response) GetDeactivateOn() string {
	if x != nil {
		return x.DeactivateOn
	}
	return ""
}

func (x *Product_Response) GetDeleted() bool {
	if x != nil {
		return x.Deleted
	}
	return false
}

func (x *Product_Response) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *Product_Response) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Product_Response) GetImages() []string {
	if x != nil {
		return x.Images
	}
	return nil
}

func (x *Product_Response) GetLivemode() bool {
	if x != nil {
		return x.Livemode
	}
	return false
}

func (x *Product_Response) GetMetadata() map[string]string {
	if x != nil {
		return x.Metadata
	}
	return nil
}

func (x *Product_Response) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Product_Response) GetObject() string {
	if x != nil {
		return x.Object
	}
	return ""
}

func (x *Product_Response) GetPackageDimensions() *PackageDimensions {
	if x != nil {
		return x.PackageDimensions
	}
	return nil
}

func (x *Product_Response) GetShippable() bool {
	if x != nil {
		return x.Shippable
	}
	return false
}

func (x *Product_Response) GetStatementDescriptor() string {
	if x != nil {
		return x.StatementDescriptor
	}
	return ""
}

func (x *Product_Response) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *Product_Response) GetUnitLabel() string {
	if x != nil {
		return x.UnitLabel
	}
	return ""
}

func (x *Product_Response) GetUpdated() int32 {
	if x != nil {
		return x.Updated
	}
	return 0
}

func (x *Product_Response) GetUrl() string {
	if x != nil {
		return x.Url
	}
	return ""
}

type UpdateProduct_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ProductId           string             `protobuf:"bytes,1,opt,name=product_id,json=productId,proto3" json:"product_id,omitempty"`
	PlanName            bool               `protobuf:"varint,2,opt,name=plan_name,json=planName,proto3" json:"plan_name,omitempty"`
	Attributes          []string           `protobuf:"bytes,3,rep,name=attributes,proto3" json:"attributes,omitempty"`
	Caption             string             `protobuf:"bytes,4,opt,name=caption,proto3" json:"caption,omitempty"`
	DeactivateOn        string             `protobuf:"bytes,5,opt,name=deactivate_on,json=deactivateOn,proto3" json:"deactivate_on,omitempty"`
	Description         string             `protobuf:"bytes,6,opt,name=description,proto3" json:"description,omitempty"`
	Id                  string             `protobuf:"bytes,7,opt,name=id,proto3" json:"id,omitempty"`
	Images              []string           `protobuf:"bytes,8,rep,name=images,proto3" json:"images,omitempty"`
	Name                string             `protobuf:"bytes,9,opt,name=name,proto3" json:"name,omitempty"`
	PackageDimensions   *PackageDimensions `protobuf:"bytes,10,opt,name=package_dimensions,json=packageDimensions,proto3" json:"package_dimensions,omitempty"`
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
		mi := &file_billing_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateProduct_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateProduct_Request) ProtoMessage() {}

func (x *UpdateProduct_Request) ProtoReflect() protoreflect.Message {
	mi := &file_billing_proto_msgTypes[6]
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
	return file_billing_proto_rawDescGZIP(), []int{2, 0}
}

func (x *UpdateProduct_Request) GetProductId() string {
	if x != nil {
		return x.ProductId
	}
	return ""
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
		mi := &file_billing_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateProduct_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateProduct_Response) ProtoMessage() {}

func (x *UpdateProduct_Response) ProtoReflect() protoreflect.Message {
	mi := &file_billing_proto_msgTypes[7]
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
	return file_billing_proto_rawDescGZIP(), []int{2, 1}
}

var File_billing_proto protoreflect.FileDescriptor

var file_billing_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x07, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x1a, 0x17, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61,
	0x74, 0x65, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0x71, 0x0a, 0x11, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x44, 0x69, 0x6d, 0x65,
	0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x02, 0x52, 0x06, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x16,
	0x0a, 0x06, 0x6c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x02, 0x52, 0x06,
	0x6c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x12, 0x16, 0x0a, 0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x02, 0x52, 0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x14,
	0x0a, 0x05, 0x77, 0x69, 0x64, 0x74, 0x68, 0x18, 0x04, 0x20, 0x01, 0x28, 0x02, 0x52, 0x05, 0x77,
	0x69, 0x64, 0x74, 0x68, 0x22, 0x84, 0x06, 0x0a, 0x07, 0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74,
	0x1a, 0x32, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x27, 0x0a, 0x0a, 0x70,
	0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42,
	0x08, 0xfa, 0x42, 0x05, 0x72, 0x03, 0xb0, 0x01, 0x01, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x64, 0x75,
	0x63, 0x74, 0x49, 0x64, 0x1a, 0xc4, 0x05, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x16, 0x0a, 0x06, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x06, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x61, 0x74, 0x74,
	0x72, 0x69, 0x62, 0x75, 0x74, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0a, 0x61,
	0x74, 0x74, 0x72, 0x69, 0x62, 0x75, 0x74, 0x65, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x61, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x61, 0x70, 0x74,
	0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x05, 0x52, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x12, 0x23, 0x0a,
	0x0d, 0x64, 0x65, 0x61, 0x63, 0x74, 0x69, 0x76, 0x61, 0x74, 0x65, 0x5f, 0x6f, 0x6e, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x64, 0x65, 0x61, 0x63, 0x74, 0x69, 0x76, 0x61, 0x74, 0x65,
	0x4f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x07, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x12, 0x20, 0x0a, 0x0b,
	0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x07, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x0e,
	0x0a, 0x02, 0x69, 0x64, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x16,
	0x0a, 0x06, 0x69, 0x6d, 0x61, 0x67, 0x65, 0x73, 0x18, 0x09, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06,
	0x69, 0x6d, 0x61, 0x67, 0x65, 0x73, 0x12, 0x1a, 0x0a, 0x08, 0x6c, 0x69, 0x76, 0x65, 0x6d, 0x6f,
	0x64, 0x65, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x6c, 0x69, 0x76, 0x65, 0x6d, 0x6f,
	0x64, 0x65, 0x12, 0x43, 0x0a, 0x08, 0x6d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x18, 0x0b,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x27, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x50,
	0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x2e,
	0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x08, 0x6d,
	0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x0c, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x6f,
	0x62, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6f, 0x62, 0x6a,
	0x65, 0x63, 0x74, 0x12, 0x49, 0x0a, 0x12, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x64,
	0x69, 0x6d, 0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x1a, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67,
	0x65, 0x44, 0x69, 0x6d, 0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x11, 0x70, 0x61, 0x63,
	0x6b, 0x61, 0x67, 0x65, 0x44, 0x69, 0x6d, 0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x1c,
	0x0a, 0x09, 0x73, 0x68, 0x69, 0x70, 0x70, 0x61, 0x62, 0x6c, 0x65, 0x18, 0x0f, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x09, 0x73, 0x68, 0x69, 0x70, 0x70, 0x61, 0x62, 0x6c, 0x65, 0x12, 0x31, 0x0a, 0x14,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x5f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x6f, 0x72, 0x18, 0x10, 0x20, 0x01, 0x28, 0x09, 0x52, 0x13, 0x73, 0x74, 0x61, 0x74,
	0x65, 0x6d, 0x65, 0x6e, 0x74, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x12,
	0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x11, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74,
	0x79, 0x70, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x75, 0x6e, 0x69, 0x74, 0x5f, 0x6c, 0x61, 0x62, 0x65,
	0x6c, 0x18, 0x12, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x75, 0x6e, 0x69, 0x74, 0x4c, 0x61, 0x62,
	0x65, 0x6c, 0x12, 0x18, 0x0a, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x18, 0x13, 0x20,
	0x01, 0x28, 0x05, 0x52, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x12, 0x10, 0x0a, 0x03,
	0x75, 0x72, 0x6c, 0x18, 0x14, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x6c, 0x1a, 0x3b,
	0x0a, 0x0d, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12,
	0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65,
	0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0xa6, 0x04, 0x0a, 0x0d,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x1a, 0x88, 0x04,
	0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x27, 0x0a, 0x0a, 0x70, 0x72, 0x6f,
	0x64, 0x75, 0x63, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x08, 0xfa,
	0x42, 0x05, 0x72, 0x03, 0xb0, 0x01, 0x01, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74,
	0x49, 0x64, 0x12, 0x1b, 0x0a, 0x09, 0x70, 0x6c, 0x61, 0x6e, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x6e, 0x4e, 0x61, 0x6d, 0x65, 0x12,
	0x1e, 0x0a, 0x0a, 0x61, 0x74, 0x74, 0x72, 0x69, 0x62, 0x75, 0x74, 0x65, 0x73, 0x18, 0x03, 0x20,
	0x03, 0x28, 0x09, 0x52, 0x0a, 0x61, 0x74, 0x74, 0x72, 0x69, 0x62, 0x75, 0x74, 0x65, 0x73, 0x12,
	0x18, 0x0a, 0x07, 0x63, 0x61, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x63, 0x61, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x23, 0x0a, 0x0d, 0x64, 0x65, 0x61,
	0x63, 0x74, 0x69, 0x76, 0x61, 0x74, 0x65, 0x5f, 0x6f, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0c, 0x64, 0x65, 0x61, 0x63, 0x74, 0x69, 0x76, 0x61, 0x74, 0x65, 0x4f, 0x6e, 0x12, 0x20,
	0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64,
	0x12, 0x16, 0x0a, 0x06, 0x69, 0x6d, 0x61, 0x67, 0x65, 0x73, 0x18, 0x08, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x06, 0x69, 0x6d, 0x61, 0x67, 0x65, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x09, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x49, 0x0a, 0x12,
	0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x64, 0x69, 0x6d, 0x65, 0x6e, 0x73, 0x69, 0x6f,
	0x6e, 0x73, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69,
	0x6e, 0x67, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x44, 0x69, 0x6d, 0x65, 0x6e, 0x73,
	0x69, 0x6f, 0x6e, 0x73, 0x52, 0x11, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x44, 0x69, 0x6d,
	0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x1c, 0x0a, 0x09, 0x73, 0x68, 0x69, 0x70, 0x70,
	0x61, 0x62, 0x6c, 0x65, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x73, 0x68, 0x69, 0x70,
	0x70, 0x61, 0x62, 0x6c, 0x65, 0x12, 0x31, 0x0a, 0x14, 0x73, 0x74, 0x61, 0x74, 0x65, 0x6d, 0x65,
	0x6e, 0x74, 0x5f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x18, 0x0c, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x13, 0x73, 0x74, 0x61, 0x74, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x44, 0x65,
	0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x12, 0x19, 0x0a, 0x08, 0x74, 0x61, 0x78, 0x5f,
	0x63, 0x6f, 0x64, 0x65, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x74, 0x61, 0x78, 0x43,
	0x6f, 0x64, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x0e, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x75, 0x6e, 0x69, 0x74, 0x5f,
	0x6c, 0x61, 0x62, 0x65, 0x6c, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x75, 0x6e, 0x69,
	0x74, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x10, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x6c, 0x1a, 0x0a, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x32, 0xa7, 0x01, 0x0a, 0x0f, 0x42, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67,
	0x48, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x72, 0x73, 0x12, 0x40, 0x0a, 0x07, 0x50, 0x72, 0x6f, 0x64,
	0x75, 0x63, 0x74, 0x12, 0x18, 0x2e, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x50, 0x72,
	0x6f, 0x64, 0x75, 0x63, 0x74, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x19, 0x2e,
	0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x2e,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x52, 0x0a, 0x0d, 0x55, 0x70,
	0x64, 0x61, 0x74, 0x65, 0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x12, 0x1e, 0x2e, 0x62, 0x69,
	0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x50, 0x72, 0x6f, 0x64,
	0x75, 0x63, 0x74, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1f, 0x2e, 0x62, 0x69,
	0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x50, 0x72, 0x6f, 0x64,
	0x75, 0x63, 0x74, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x2c,
	0x5a, 0x2a, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x77, 0x65, 0x72,
	0x62, 0x6f, 0x74, 0x2f, 0x77, 0x65, 0x72, 0x62, 0x6f, 0x74, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x62, 0x69, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
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

var file_billing_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_billing_proto_goTypes = []interface{}{
	(*PackageDimensions)(nil),      // 0: billing.PackageDimensions
	(*Product)(nil),                // 1: billing.Product
	(*UpdateProduct)(nil),          // 2: billing.UpdateProduct
	(*Product_Request)(nil),        // 3: billing.Product.Request
	(*Product_Response)(nil),       // 4: billing.Product.Response
	nil,                            // 5: billing.Product.Response.MetadataEntry
	(*UpdateProduct_Request)(nil),  // 6: billing.UpdateProduct.Request
	(*UpdateProduct_Response)(nil), // 7: billing.UpdateProduct.Response
}
var file_billing_proto_depIdxs = []int32{
	5, // 0: billing.Product.Response.metadata:type_name -> billing.Product.Response.MetadataEntry
	0, // 1: billing.Product.Response.package_dimensions:type_name -> billing.PackageDimensions
	0, // 2: billing.UpdateProduct.Request.package_dimensions:type_name -> billing.PackageDimensions
	3, // 3: billing.BillingHandlers.Product:input_type -> billing.Product.Request
	6, // 4: billing.BillingHandlers.UpdateProduct:input_type -> billing.UpdateProduct.Request
	4, // 5: billing.BillingHandlers.Product:output_type -> billing.Product.Response
	7, // 6: billing.BillingHandlers.UpdateProduct:output_type -> billing.UpdateProduct.Response
	5, // [5:7] is the sub-list for method output_type
	3, // [3:5] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
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
			switch v := v.(*Product); i {
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
		file_billing_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Product_Request); i {
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
		file_billing_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Product_Response); i {
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
		file_billing_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
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
		file_billing_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
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
			NumMessages:   8,
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
