// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// source: system.proto

package system

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	SystemHandlers_UserMetrics_FullMethodName = "/system.SystemHandlers/UserMetrics"
	SystemHandlers_Countries_FullMethodName   = "/system.SystemHandlers/Countries"
	SystemHandlers_CountryByIP_FullMethodName = "/system.SystemHandlers/CountryByIP"
)

// SystemHandlersClient is the client API for SystemHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SystemHandlersClient interface {
	UserMetrics(ctx context.Context, in *UserMetrics_Request, opts ...grpc.CallOption) (*UserMetrics_Response, error)
	Countries(ctx context.Context, in *Countries_Request, opts ...grpc.CallOption) (*Countries_Response, error)
	CountryByIP(ctx context.Context, in *CountryByIP_Request, opts ...grpc.CallOption) (*CountryByIP_Response, error)
}

type systemHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewSystemHandlersClient(cc grpc.ClientConnInterface) SystemHandlersClient {
	return &systemHandlersClient{cc}
}

func (c *systemHandlersClient) UserMetrics(ctx context.Context, in *UserMetrics_Request, opts ...grpc.CallOption) (*UserMetrics_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UserMetrics_Response)
	err := c.cc.Invoke(ctx, SystemHandlers_UserMetrics_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *systemHandlersClient) Countries(ctx context.Context, in *Countries_Request, opts ...grpc.CallOption) (*Countries_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Countries_Response)
	err := c.cc.Invoke(ctx, SystemHandlers_Countries_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *systemHandlersClient) CountryByIP(ctx context.Context, in *CountryByIP_Request, opts ...grpc.CallOption) (*CountryByIP_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CountryByIP_Response)
	err := c.cc.Invoke(ctx, SystemHandlers_CountryByIP_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SystemHandlersServer is the server API for SystemHandlers service.
// All implementations must embed UnimplementedSystemHandlersServer
// for forward compatibility.
type SystemHandlersServer interface {
	UserMetrics(context.Context, *UserMetrics_Request) (*UserMetrics_Response, error)
	Countries(context.Context, *Countries_Request) (*Countries_Response, error)
	CountryByIP(context.Context, *CountryByIP_Request) (*CountryByIP_Response, error)
	mustEmbedUnimplementedSystemHandlersServer()
}

// UnimplementedSystemHandlersServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedSystemHandlersServer struct{}

func (UnimplementedSystemHandlersServer) UserMetrics(context.Context, *UserMetrics_Request) (*UserMetrics_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UserMetrics not implemented")
}
func (UnimplementedSystemHandlersServer) Countries(context.Context, *Countries_Request) (*Countries_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Countries not implemented")
}
func (UnimplementedSystemHandlersServer) CountryByIP(context.Context, *CountryByIP_Request) (*CountryByIP_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CountryByIP not implemented")
}
func (UnimplementedSystemHandlersServer) mustEmbedUnimplementedSystemHandlersServer() {}
func (UnimplementedSystemHandlersServer) testEmbeddedByValue()                        {}

// UnsafeSystemHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SystemHandlersServer will
// result in compilation errors.
type UnsafeSystemHandlersServer interface {
	mustEmbedUnimplementedSystemHandlersServer()
}

func RegisterSystemHandlersServer(s grpc.ServiceRegistrar, srv SystemHandlersServer) {
	// If the following call pancis, it indicates UnimplementedSystemHandlersServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&SystemHandlers_ServiceDesc, srv)
}

func _SystemHandlers_UserMetrics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UserMetrics_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SystemHandlersServer).UserMetrics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SystemHandlers_UserMetrics_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SystemHandlersServer).UserMetrics(ctx, req.(*UserMetrics_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SystemHandlers_Countries_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Countries_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SystemHandlersServer).Countries(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SystemHandlers_Countries_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SystemHandlersServer).Countries(ctx, req.(*Countries_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SystemHandlers_CountryByIP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CountryByIP_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SystemHandlersServer).CountryByIP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SystemHandlers_CountryByIP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SystemHandlersServer).CountryByIP(ctx, req.(*CountryByIP_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// SystemHandlers_ServiceDesc is the grpc.ServiceDesc for SystemHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SystemHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "system.SystemHandlers",
	HandlerType: (*SystemHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "UserMetrics",
			Handler:    _SystemHandlers_UserMetrics_Handler,
		},
		{
			MethodName: "Countries",
			Handler:    _SystemHandlers_Countries_Handler,
		},
		{
			MethodName: "CountryByIP",
			Handler:    _SystemHandlers_CountryByIP_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "system.proto",
}