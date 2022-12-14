// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.7
// source: info.proto

package info

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

// InfoHandlersClient is the client API for InfoHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type InfoHandlersClient interface {
	UserStatistics(ctx context.Context, in *UserStatistics_Request, opts ...grpc.CallOption) (*UserStatistics_Response, error)
}

type infoHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewInfoHandlersClient(cc grpc.ClientConnInterface) InfoHandlersClient {
	return &infoHandlersClient{cc}
}

func (c *infoHandlersClient) UserStatistics(ctx context.Context, in *UserStatistics_Request, opts ...grpc.CallOption) (*UserStatistics_Response, error) {
	out := new(UserStatistics_Response)
	err := c.cc.Invoke(ctx, "/info.InfoHandlers/UserStatistics", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// InfoHandlersServer is the server API for InfoHandlers service.
// All implementations must embed UnimplementedInfoHandlersServer
// for forward compatibility
type InfoHandlersServer interface {
	UserStatistics(context.Context, *UserStatistics_Request) (*UserStatistics_Response, error)
	mustEmbedUnimplementedInfoHandlersServer()
}

// UnimplementedInfoHandlersServer must be embedded to have forward compatible implementations.
type UnimplementedInfoHandlersServer struct {
}

func (UnimplementedInfoHandlersServer) UserStatistics(context.Context, *UserStatistics_Request) (*UserStatistics_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UserStatistics not implemented")
}
func (UnimplementedInfoHandlersServer) mustEmbedUnimplementedInfoHandlersServer() {}

// UnsafeInfoHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to InfoHandlersServer will
// result in compilation errors.
type UnsafeInfoHandlersServer interface {
	mustEmbedUnimplementedInfoHandlersServer()
}

func RegisterInfoHandlersServer(s grpc.ServiceRegistrar, srv InfoHandlersServer) {
	s.RegisterService(&InfoHandlers_ServiceDesc, srv)
}

func _InfoHandlers_UserStatistics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UserStatistics_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InfoHandlersServer).UserStatistics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/info.InfoHandlers/UserStatistics",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InfoHandlersServer).UserStatistics(ctx, req.(*UserStatistics_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// InfoHandlers_ServiceDesc is the grpc.ServiceDesc for InfoHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var InfoHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "info.InfoHandlers",
	HandlerType: (*InfoHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "UserStatistics",
			Handler:    _InfoHandlers_UserStatistics_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "info.proto",
}
