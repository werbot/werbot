// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.1
// source: event.proto

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
	EventHandlers_Events_FullMethodName   = "/event.EventHandlers/Events"
	EventHandlers_Event_FullMethodName    = "/event.EventHandlers/Event"
	EventHandlers_AddEvent_FullMethodName = "/event.EventHandlers/AddEvent"
)

// EventHandlersClient is the client API for EventHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type EventHandlersClient interface {
	Events(ctx context.Context, in *Events_Request, opts ...grpc.CallOption) (*Events_Response, error)
	Event(ctx context.Context, in *Event_Request, opts ...grpc.CallOption) (*Event_Response, error)
	AddEvent(ctx context.Context, in *AddEvent_Request, opts ...grpc.CallOption) (*AddEvent_Response, error)
}

type eventHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewEventHandlersClient(cc grpc.ClientConnInterface) EventHandlersClient {
	return &eventHandlersClient{cc}
}

func (c *eventHandlersClient) Events(ctx context.Context, in *Events_Request, opts ...grpc.CallOption) (*Events_Response, error) {
	out := new(Events_Response)
	err := c.cc.Invoke(ctx, EventHandlers_Events_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *eventHandlersClient) Event(ctx context.Context, in *Event_Request, opts ...grpc.CallOption) (*Event_Response, error) {
	out := new(Event_Response)
	err := c.cc.Invoke(ctx, EventHandlers_Event_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *eventHandlersClient) AddEvent(ctx context.Context, in *AddEvent_Request, opts ...grpc.CallOption) (*AddEvent_Response, error) {
	out := new(AddEvent_Response)
	err := c.cc.Invoke(ctx, EventHandlers_AddEvent_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EventHandlersServer is the server API for EventHandlers service.
// All implementations must embed UnimplementedEventHandlersServer
// for forward compatibility
type EventHandlersServer interface {
	Events(context.Context, *Events_Request) (*Events_Response, error)
	Event(context.Context, *Event_Request) (*Event_Response, error)
	AddEvent(context.Context, *AddEvent_Request) (*AddEvent_Response, error)
	mustEmbedUnimplementedEventHandlersServer()
}

// UnimplementedEventHandlersServer must be embedded to have forward compatible implementations.
type UnimplementedEventHandlersServer struct {
}

func (UnimplementedEventHandlersServer) Events(context.Context, *Events_Request) (*Events_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Events not implemented")
}
func (UnimplementedEventHandlersServer) Event(context.Context, *Event_Request) (*Event_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Event not implemented")
}
func (UnimplementedEventHandlersServer) AddEvent(context.Context, *AddEvent_Request) (*AddEvent_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddEvent not implemented")
}
func (UnimplementedEventHandlersServer) mustEmbedUnimplementedEventHandlersServer() {}

// UnsafeEventHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to EventHandlersServer will
// result in compilation errors.
type UnsafeEventHandlersServer interface {
	mustEmbedUnimplementedEventHandlersServer()
}

func RegisterEventHandlersServer(s grpc.ServiceRegistrar, srv EventHandlersServer) {
	s.RegisterService(&EventHandlers_ServiceDesc, srv)
}

func _EventHandlers_Events_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Events_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EventHandlersServer).Events(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: EventHandlers_Events_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EventHandlersServer).Events(ctx, req.(*Events_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _EventHandlers_Event_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Event_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EventHandlersServer).Event(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: EventHandlers_Event_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EventHandlersServer).Event(ctx, req.(*Event_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _EventHandlers_AddEvent_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddEvent_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EventHandlersServer).AddEvent(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: EventHandlers_AddEvent_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EventHandlersServer).AddEvent(ctx, req.(*AddEvent_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// EventHandlers_ServiceDesc is the grpc.ServiceDesc for EventHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var EventHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "event.EventHandlers",
	HandlerType: (*EventHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Events",
			Handler:    _EventHandlers_Events_Handler,
		},
		{
			MethodName: "Event",
			Handler:    _EventHandlers_Event_Handler,
		},
		{
			MethodName: "AddEvent",
			Handler:    _EventHandlers_AddEvent_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "event.proto",
}
