// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// source: notification.proto

package notification

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
	NotificationHandlers_SendMail_FullMethodName = "/notification.NotificationHandlers/SendMail"
)

// NotificationHandlersClient is the client API for NotificationHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type NotificationHandlersClient interface {
	SendMail(ctx context.Context, in *SendMail_Request, opts ...grpc.CallOption) (*SendMail_Response, error)
}

type notificationHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewNotificationHandlersClient(cc grpc.ClientConnInterface) NotificationHandlersClient {
	return &notificationHandlersClient{cc}
}

func (c *notificationHandlersClient) SendMail(ctx context.Context, in *SendMail_Request, opts ...grpc.CallOption) (*SendMail_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SendMail_Response)
	err := c.cc.Invoke(ctx, NotificationHandlers_SendMail_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NotificationHandlersServer is the server API for NotificationHandlers service.
// All implementations must embed UnimplementedNotificationHandlersServer
// for forward compatibility.
type NotificationHandlersServer interface {
	SendMail(context.Context, *SendMail_Request) (*SendMail_Response, error)
	mustEmbedUnimplementedNotificationHandlersServer()
}

// UnimplementedNotificationHandlersServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedNotificationHandlersServer struct{}

func (UnimplementedNotificationHandlersServer) SendMail(context.Context, *SendMail_Request) (*SendMail_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendMail not implemented")
}
func (UnimplementedNotificationHandlersServer) mustEmbedUnimplementedNotificationHandlersServer() {}
func (UnimplementedNotificationHandlersServer) testEmbeddedByValue()                              {}

// UnsafeNotificationHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to NotificationHandlersServer will
// result in compilation errors.
type UnsafeNotificationHandlersServer interface {
	mustEmbedUnimplementedNotificationHandlersServer()
}

func RegisterNotificationHandlersServer(s grpc.ServiceRegistrar, srv NotificationHandlersServer) {
	// If the following call pancis, it indicates UnimplementedNotificationHandlersServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&NotificationHandlers_ServiceDesc, srv)
}

func _NotificationHandlers_SendMail_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendMail_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NotificationHandlersServer).SendMail(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: NotificationHandlers_SendMail_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NotificationHandlersServer).SendMail(ctx, req.(*SendMail_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// NotificationHandlers_ServiceDesc is the grpc.ServiceDesc for NotificationHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var NotificationHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "notification.NotificationHandlers",
	HandlerType: (*NotificationHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SendMail",
			Handler:    _NotificationHandlers_SendMail_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "notification.proto",
}
