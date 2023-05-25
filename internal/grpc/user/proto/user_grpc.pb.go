// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.23.1
// source: user.proto

package proto

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

const (
	UserHandlers_ListUsers_FullMethodName      = "/user.UserHandlers/ListUsers"
	UserHandlers_User_FullMethodName           = "/user.UserHandlers/User"
	UserHandlers_AddUser_FullMethodName        = "/user.UserHandlers/AddUser"
	UserHandlers_UpdateUser_FullMethodName     = "/user.UserHandlers/UpdateUser"
	UserHandlers_DeleteUser_FullMethodName     = "/user.UserHandlers/DeleteUser"
	UserHandlers_UpdatePassword_FullMethodName = "/user.UserHandlers/UpdatePassword"
)

// UserHandlersClient is the client API for UserHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type UserHandlersClient interface {
	// User section
	ListUsers(ctx context.Context, in *ListUsers_Request, opts ...grpc.CallOption) (*ListUsers_Response, error)
	User(ctx context.Context, in *User_Request, opts ...grpc.CallOption) (*User_Response, error)
	AddUser(ctx context.Context, in *AddUser_Request, opts ...grpc.CallOption) (*AddUser_Response, error)
	UpdateUser(ctx context.Context, in *UpdateUser_Request, opts ...grpc.CallOption) (*UpdateUser_Response, error)
	DeleteUser(ctx context.Context, in *DeleteUser_Request, opts ...grpc.CallOption) (*DeleteUser_Response, error)
	UpdatePassword(ctx context.Context, in *UpdatePassword_Request, opts ...grpc.CallOption) (*UpdatePassword_Response, error)
}

type userHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewUserHandlersClient(cc grpc.ClientConnInterface) UserHandlersClient {
	return &userHandlersClient{cc}
}

func (c *userHandlersClient) ListUsers(ctx context.Context, in *ListUsers_Request, opts ...grpc.CallOption) (*ListUsers_Response, error) {
	out := new(ListUsers_Response)
	err := c.cc.Invoke(ctx, UserHandlers_ListUsers_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *userHandlersClient) User(ctx context.Context, in *User_Request, opts ...grpc.CallOption) (*User_Response, error) {
	out := new(User_Response)
	err := c.cc.Invoke(ctx, UserHandlers_User_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *userHandlersClient) AddUser(ctx context.Context, in *AddUser_Request, opts ...grpc.CallOption) (*AddUser_Response, error) {
	out := new(AddUser_Response)
	err := c.cc.Invoke(ctx, UserHandlers_AddUser_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *userHandlersClient) UpdateUser(ctx context.Context, in *UpdateUser_Request, opts ...grpc.CallOption) (*UpdateUser_Response, error) {
	out := new(UpdateUser_Response)
	err := c.cc.Invoke(ctx, UserHandlers_UpdateUser_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *userHandlersClient) DeleteUser(ctx context.Context, in *DeleteUser_Request, opts ...grpc.CallOption) (*DeleteUser_Response, error) {
	out := new(DeleteUser_Response)
	err := c.cc.Invoke(ctx, UserHandlers_DeleteUser_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *userHandlersClient) UpdatePassword(ctx context.Context, in *UpdatePassword_Request, opts ...grpc.CallOption) (*UpdatePassword_Response, error) {
	out := new(UpdatePassword_Response)
	err := c.cc.Invoke(ctx, UserHandlers_UpdatePassword_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UserHandlersServer is the server API for UserHandlers service.
// All implementations must embed UnimplementedUserHandlersServer
// for forward compatibility
type UserHandlersServer interface {
	// User section
	ListUsers(context.Context, *ListUsers_Request) (*ListUsers_Response, error)
	User(context.Context, *User_Request) (*User_Response, error)
	AddUser(context.Context, *AddUser_Request) (*AddUser_Response, error)
	UpdateUser(context.Context, *UpdateUser_Request) (*UpdateUser_Response, error)
	DeleteUser(context.Context, *DeleteUser_Request) (*DeleteUser_Response, error)
	UpdatePassword(context.Context, *UpdatePassword_Request) (*UpdatePassword_Response, error)
	mustEmbedUnimplementedUserHandlersServer()
}

// UnimplementedUserHandlersServer must be embedded to have forward compatible implementations.
type UnimplementedUserHandlersServer struct {
}

func (UnimplementedUserHandlersServer) ListUsers(context.Context, *ListUsers_Request) (*ListUsers_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListUsers not implemented")
}
func (UnimplementedUserHandlersServer) User(context.Context, *User_Request) (*User_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method User not implemented")
}
func (UnimplementedUserHandlersServer) AddUser(context.Context, *AddUser_Request) (*AddUser_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddUser not implemented")
}
func (UnimplementedUserHandlersServer) UpdateUser(context.Context, *UpdateUser_Request) (*UpdateUser_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateUser not implemented")
}
func (UnimplementedUserHandlersServer) DeleteUser(context.Context, *DeleteUser_Request) (*DeleteUser_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteUser not implemented")
}
func (UnimplementedUserHandlersServer) UpdatePassword(context.Context, *UpdatePassword_Request) (*UpdatePassword_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdatePassword not implemented")
}
func (UnimplementedUserHandlersServer) mustEmbedUnimplementedUserHandlersServer() {}

// UnsafeUserHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to UserHandlersServer will
// result in compilation errors.
type UnsafeUserHandlersServer interface {
	mustEmbedUnimplementedUserHandlersServer()
}

func RegisterUserHandlersServer(s grpc.ServiceRegistrar, srv UserHandlersServer) {
	s.RegisterService(&UserHandlers_ServiceDesc, srv)
}

func _UserHandlers_ListUsers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListUsers_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UserHandlersServer).ListUsers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UserHandlers_ListUsers_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UserHandlersServer).ListUsers(ctx, req.(*ListUsers_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _UserHandlers_User_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(User_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UserHandlersServer).User(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UserHandlers_User_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UserHandlersServer).User(ctx, req.(*User_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _UserHandlers_AddUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddUser_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UserHandlersServer).AddUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UserHandlers_AddUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UserHandlersServer).AddUser(ctx, req.(*AddUser_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _UserHandlers_UpdateUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateUser_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UserHandlersServer).UpdateUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UserHandlers_UpdateUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UserHandlersServer).UpdateUser(ctx, req.(*UpdateUser_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _UserHandlers_DeleteUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteUser_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UserHandlersServer).DeleteUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UserHandlers_DeleteUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UserHandlersServer).DeleteUser(ctx, req.(*DeleteUser_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _UserHandlers_UpdatePassword_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdatePassword_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UserHandlersServer).UpdatePassword(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UserHandlers_UpdatePassword_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UserHandlersServer).UpdatePassword(ctx, req.(*UpdatePassword_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// UserHandlers_ServiceDesc is the grpc.ServiceDesc for UserHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var UserHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "user.UserHandlers",
	HandlerType: (*UserHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListUsers",
			Handler:    _UserHandlers_ListUsers_Handler,
		},
		{
			MethodName: "User",
			Handler:    _UserHandlers_User_Handler,
		},
		{
			MethodName: "AddUser",
			Handler:    _UserHandlers_AddUser_Handler,
		},
		{
			MethodName: "UpdateUser",
			Handler:    _UserHandlers_UpdateUser_Handler,
		},
		{
			MethodName: "DeleteUser",
			Handler:    _UserHandlers_DeleteUser_Handler,
		},
		{
			MethodName: "UpdatePassword",
			Handler:    _UserHandlers_UpdatePassword_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "user.proto",
}
