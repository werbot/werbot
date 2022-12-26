// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.7
// source: key.proto

package key

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// KeyHandlersClient is the client API for KeyHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type KeyHandlersClient interface {
	ListPublicKeys(ctx context.Context, in *ListPublicKeys_Request, opts ...grpc.CallOption) (*ListPublicKeys_Response, error)
	PublicKey(ctx context.Context, in *PublicKey_Request, opts ...grpc.CallOption) (*PublicKey_Response, error)
	AddPublicKey(ctx context.Context, in *AddPublicKey_Request, opts ...grpc.CallOption) (*AddPublicKey_Response, error)
	UpdatePublicKey(ctx context.Context, in *UpdatePublicKey_Request, opts ...grpc.CallOption) (*UpdatePublicKey_Response, error)
	DeletePublicKey(ctx context.Context, in *DeletePublicKey_Request, opts ...grpc.CallOption) (*DeletePublicKey_Response, error)
	GenerateSSHKey(ctx context.Context, in *GenerateSSHKey_Request, opts ...grpc.CallOption) (*GenerateSSHKey_Response, error)
}

type keyHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewKeyHandlersClient(cc grpc.ClientConnInterface) KeyHandlersClient {
	return &keyHandlersClient{cc}
}

func (c *keyHandlersClient) ListPublicKeys(ctx context.Context, in *ListPublicKeys_Request, opts ...grpc.CallOption) (*ListPublicKeys_Response, error) {
	out := new(ListPublicKeys_Response)
	err := c.cc.Invoke(ctx, "/key.KeyHandlers/ListPublicKeys", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *keyHandlersClient) PublicKey(ctx context.Context, in *PublicKey_Request, opts ...grpc.CallOption) (*PublicKey_Response, error) {
	out := new(PublicKey_Response)
	err := c.cc.Invoke(ctx, "/key.KeyHandlers/PublicKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *keyHandlersClient) AddPublicKey(ctx context.Context, in *AddPublicKey_Request, opts ...grpc.CallOption) (*AddPublicKey_Response, error) {
	out := new(AddPublicKey_Response)
	err := c.cc.Invoke(ctx, "/key.KeyHandlers/AddPublicKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *keyHandlersClient) UpdatePublicKey(ctx context.Context, in *UpdatePublicKey_Request, opts ...grpc.CallOption) (*UpdatePublicKey_Response, error) {
	out := new(UpdatePublicKey_Response)
	err := c.cc.Invoke(ctx, "/key.KeyHandlers/UpdatePublicKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *keyHandlersClient) DeletePublicKey(ctx context.Context, in *DeletePublicKey_Request, opts ...grpc.CallOption) (*DeletePublicKey_Response, error) {
	out := new(DeletePublicKey_Response)
	err := c.cc.Invoke(ctx, "/key.KeyHandlers/DeletePublicKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *keyHandlersClient) GenerateSSHKey(ctx context.Context, in *GenerateSSHKey_Request, opts ...grpc.CallOption) (*GenerateSSHKey_Response, error) {
	out := new(GenerateSSHKey_Response)
	err := c.cc.Invoke(ctx, "/key.KeyHandlers/GenerateSSHKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// KeyHandlersServer is the server API for KeyHandlers service.
// All implementations must embed UnimplementedKeyHandlersServer
// for forward compatibility
type KeyHandlersServer interface {
	ListPublicKeys(context.Context, *ListPublicKeys_Request) (*ListPublicKeys_Response, error)
	PublicKey(context.Context, *PublicKey_Request) (*PublicKey_Response, error)
	AddPublicKey(context.Context, *AddPublicKey_Request) (*AddPublicKey_Response, error)
	UpdatePublicKey(context.Context, *UpdatePublicKey_Request) (*UpdatePublicKey_Response, error)
	DeletePublicKey(context.Context, *DeletePublicKey_Request) (*DeletePublicKey_Response, error)
	GenerateSSHKey(context.Context, *GenerateSSHKey_Request) (*GenerateSSHKey_Response, error)
	mustEmbedUnimplementedKeyHandlersServer()
}

// UnimplementedKeyHandlersServer must be embedded to have forward compatible implementations.
type UnimplementedKeyHandlersServer struct {
}

func (UnimplementedKeyHandlersServer) ListPublicKeys(context.Context, *ListPublicKeys_Request) (*ListPublicKeys_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListPublicKeys not implemented")
}
func (UnimplementedKeyHandlersServer) PublicKey(context.Context, *PublicKey_Request) (*PublicKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PublicKey not implemented")
}
func (UnimplementedKeyHandlersServer) AddPublicKey(context.Context, *AddPublicKey_Request) (*AddPublicKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddPublicKey not implemented")
}
func (UnimplementedKeyHandlersServer) UpdatePublicKey(context.Context, *UpdatePublicKey_Request) (*UpdatePublicKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdatePublicKey not implemented")
}
func (UnimplementedKeyHandlersServer) DeletePublicKey(context.Context, *DeletePublicKey_Request) (*DeletePublicKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeletePublicKey not implemented")
}
func (UnimplementedKeyHandlersServer) GenerateSSHKey(context.Context, *GenerateSSHKey_Request) (*GenerateSSHKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GenerateSSHKey not implemented")
}
func (UnimplementedKeyHandlersServer) mustEmbedUnimplementedKeyHandlersServer() {}

// UnsafeKeyHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to KeyHandlersServer will
// result in compilation errors.
type UnsafeKeyHandlersServer interface {
	mustEmbedUnimplementedKeyHandlersServer()
}

func RegisterKeyHandlersServer(s grpc.ServiceRegistrar, srv KeyHandlersServer) {
	s.RegisterService(&KeyHandlers_ServiceDesc, srv)
}

func _KeyHandlers_ListPublicKeys_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListPublicKeys_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KeyHandlersServer).ListPublicKeys(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/key.KeyHandlers/ListPublicKeys",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KeyHandlersServer).ListPublicKeys(ctx, req.(*ListPublicKeys_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _KeyHandlers_PublicKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PublicKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KeyHandlersServer).PublicKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/key.KeyHandlers/PublicKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KeyHandlersServer).PublicKey(ctx, req.(*PublicKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _KeyHandlers_AddPublicKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddPublicKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KeyHandlersServer).AddPublicKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/key.KeyHandlers/AddPublicKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KeyHandlersServer).AddPublicKey(ctx, req.(*AddPublicKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _KeyHandlers_UpdatePublicKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdatePublicKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KeyHandlersServer).UpdatePublicKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/key.KeyHandlers/UpdatePublicKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KeyHandlersServer).UpdatePublicKey(ctx, req.(*UpdatePublicKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _KeyHandlers_DeletePublicKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeletePublicKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KeyHandlersServer).DeletePublicKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/key.KeyHandlers/DeletePublicKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KeyHandlersServer).DeletePublicKey(ctx, req.(*DeletePublicKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _KeyHandlers_GenerateSSHKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GenerateSSHKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KeyHandlersServer).GenerateSSHKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/key.KeyHandlers/GenerateSSHKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KeyHandlersServer).GenerateSSHKey(ctx, req.(*GenerateSSHKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// KeyHandlers_ServiceDesc is the grpc.ServiceDesc for KeyHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var KeyHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "key.KeyHandlers",
	HandlerType: (*KeyHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListPublicKeys",
			Handler:    _KeyHandlers_ListPublicKeys_Handler,
		},
		{
			MethodName: "PublicKey",
			Handler:    _KeyHandlers_PublicKey_Handler,
		},
		{
			MethodName: "AddPublicKey",
			Handler:    _KeyHandlers_AddPublicKey_Handler,
		},
		{
			MethodName: "UpdatePublicKey",
			Handler:    _KeyHandlers_UpdatePublicKey_Handler,
		},
		{
			MethodName: "DeletePublicKey",
			Handler:    _KeyHandlers_DeletePublicKey_Handler,
		},
		{
			MethodName: "GenerateSSHKey",
			Handler:    _KeyHandlers_GenerateSSHKey_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "key.proto",
}
