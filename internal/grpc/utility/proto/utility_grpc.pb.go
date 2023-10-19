// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.23.3
// source: utility.proto

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
	UtilityHandlers_Countries_FullMethodName   = "/utility.UtilityHandlers/Countries"
	UtilityHandlers_CountryByIP_FullMethodName = "/utility.UtilityHandlers/CountryByIP"
)

// UtilityHandlersClient is the client API for UtilityHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type UtilityHandlersClient interface {
	Countries(ctx context.Context, in *Countries_Request, opts ...grpc.CallOption) (*Countries_Response, error)
	CountryByIP(ctx context.Context, in *CountryByIP_Request, opts ...grpc.CallOption) (*CountryByIP_Response, error)
}

type utilityHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewUtilityHandlersClient(cc grpc.ClientConnInterface) UtilityHandlersClient {
	return &utilityHandlersClient{cc}
}

func (c *utilityHandlersClient) Countries(ctx context.Context, in *Countries_Request, opts ...grpc.CallOption) (*Countries_Response, error) {
	out := new(Countries_Response)
	err := c.cc.Invoke(ctx, UtilityHandlers_Countries_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *utilityHandlersClient) CountryByIP(ctx context.Context, in *CountryByIP_Request, opts ...grpc.CallOption) (*CountryByIP_Response, error) {
	out := new(CountryByIP_Response)
	err := c.cc.Invoke(ctx, UtilityHandlers_CountryByIP_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UtilityHandlersServer is the server API for UtilityHandlers service.
// All implementations must embed UnimplementedUtilityHandlersServer
// for forward compatibility
type UtilityHandlersServer interface {
	Countries(context.Context, *Countries_Request) (*Countries_Response, error)
	CountryByIP(context.Context, *CountryByIP_Request) (*CountryByIP_Response, error)
	mustEmbedUnimplementedUtilityHandlersServer()
}

// UnimplementedUtilityHandlersServer must be embedded to have forward compatible implementations.
type UnimplementedUtilityHandlersServer struct {
}

func (UnimplementedUtilityHandlersServer) Countries(context.Context, *Countries_Request) (*Countries_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Countries not implemented")
}
func (UnimplementedUtilityHandlersServer) CountryByIP(context.Context, *CountryByIP_Request) (*CountryByIP_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CountryByIP not implemented")
}
func (UnimplementedUtilityHandlersServer) mustEmbedUnimplementedUtilityHandlersServer() {}

// UnsafeUtilityHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to UtilityHandlersServer will
// result in compilation errors.
type UnsafeUtilityHandlersServer interface {
	mustEmbedUnimplementedUtilityHandlersServer()
}

func RegisterUtilityHandlersServer(s grpc.ServiceRegistrar, srv UtilityHandlersServer) {
	s.RegisterService(&UtilityHandlers_ServiceDesc, srv)
}

func _UtilityHandlers_Countries_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Countries_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UtilityHandlersServer).Countries(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UtilityHandlers_Countries_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UtilityHandlersServer).Countries(ctx, req.(*Countries_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _UtilityHandlers_CountryByIP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CountryByIP_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UtilityHandlersServer).CountryByIP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UtilityHandlers_CountryByIP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UtilityHandlersServer).CountryByIP(ctx, req.(*CountryByIP_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// UtilityHandlers_ServiceDesc is the grpc.ServiceDesc for UtilityHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var UtilityHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "utility.UtilityHandlers",
	HandlerType: (*UtilityHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Countries",
			Handler:    _UtilityHandlers_Countries_Handler,
		},
		{
			MethodName: "CountryByIP",
			Handler:    _UtilityHandlers_CountryByIP_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "utility.proto",
}
