// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.23.3
// source: info.proto

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
	InfoHandlers_UserMetrics_FullMethodName = "/info.InfoHandlers/UserMetrics"
)

// InfoHandlersClient is the client API for InfoHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type InfoHandlersClient interface {
	UserMetrics(ctx context.Context, in *UserMetrics_Request, opts ...grpc.CallOption) (*UserMetrics_Response, error)
}

type infoHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewInfoHandlersClient(cc grpc.ClientConnInterface) InfoHandlersClient {
	return &infoHandlersClient{cc}
}

func (c *infoHandlersClient) UserMetrics(ctx context.Context, in *UserMetrics_Request, opts ...grpc.CallOption) (*UserMetrics_Response, error) {
	out := new(UserMetrics_Response)
	err := c.cc.Invoke(ctx, InfoHandlers_UserMetrics_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// InfoHandlersServer is the server API for InfoHandlers service.
// All implementations must embed UnimplementedInfoHandlersServer
// for forward compatibility
type InfoHandlersServer interface {
	UserMetrics(context.Context, *UserMetrics_Request) (*UserMetrics_Response, error)
	mustEmbedUnimplementedInfoHandlersServer()
}

// UnimplementedInfoHandlersServer must be embedded to have forward compatible implementations.
type UnimplementedInfoHandlersServer struct {
}

func (UnimplementedInfoHandlersServer) UserMetrics(context.Context, *UserMetrics_Request) (*UserMetrics_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UserMetrics not implemented")
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

func _InfoHandlers_UserMetrics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UserMetrics_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InfoHandlersServer).UserMetrics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: InfoHandlers_UserMetrics_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InfoHandlersServer).UserMetrics(ctx, req.(*UserMetrics_Request))
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
			MethodName: "UserMetrics",
			Handler:    _InfoHandlers_UserMetrics_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "info.proto",
}
