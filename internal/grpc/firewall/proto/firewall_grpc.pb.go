// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.1
// source: firewall.proto

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
	FirewallHandlers_IPAccess_FullMethodName             = "/firewall.FirewallHandlers/IPAccess"
	FirewallHandlers_ServerFirewall_FullMethodName       = "/firewall.FirewallHandlers/ServerFirewall"
	FirewallHandlers_AddServerFirewall_FullMethodName    = "/firewall.FirewallHandlers/AddServerFirewall"
	FirewallHandlers_UpdateServerFirewall_FullMethodName = "/firewall.FirewallHandlers/UpdateServerFirewall"
	FirewallHandlers_DeleteServerFirewall_FullMethodName = "/firewall.FirewallHandlers/DeleteServerFirewall"
	FirewallHandlers_ServerAccess_FullMethodName         = "/firewall.FirewallHandlers/ServerAccess"
	FirewallHandlers_ServerAccessUser_FullMethodName     = "/firewall.FirewallHandlers/ServerAccessUser"
	FirewallHandlers_ServerAccessTime_FullMethodName     = "/firewall.FirewallHandlers/ServerAccessTime"
	FirewallHandlers_ServerAccessIP_FullMethodName       = "/firewall.FirewallHandlers/ServerAccessIP"
	FirewallHandlers_ServerAccessCountry_FullMethodName  = "/firewall.FirewallHandlers/ServerAccessCountry"
)

// FirewallHandlersClient is the client API for FirewallHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type FirewallHandlersClient interface {
	IPAccess(ctx context.Context, in *IPAccess_Request, opts ...grpc.CallOption) (*IPAccess_Response, error)
	// Server firewall section
	ServerFirewall(ctx context.Context, in *ServerFirewall_Request, opts ...grpc.CallOption) (*ServerFirewall_Response, error)
	AddServerFirewall(ctx context.Context, in *AddServerFirewall_Request, opts ...grpc.CallOption) (*AddServerFirewall_Response, error)
	UpdateServerFirewall(ctx context.Context, in *UpdateServerFirewall_Request, opts ...grpc.CallOption) (*UpdateServerFirewall_Response, error)
	DeleteServerFirewall(ctx context.Context, in *DeleteServerFirewall_Request, opts ...grpc.CallOption) (*DeleteServerFirewall_Response, error)
	// Server access section
	ServerAccess(ctx context.Context, in *ServerAccess_Request, opts ...grpc.CallOption) (*ServerAccess_Response, error)
	ServerAccessUser(ctx context.Context, in *ServerAccessUser_Request, opts ...grpc.CallOption) (*ServerAccessUser_Response, error)
	ServerAccessTime(ctx context.Context, in *ServerAccessTime_Request, opts ...grpc.CallOption) (*ServerAccessTime_Response, error)
	ServerAccessIP(ctx context.Context, in *ServerAccessIP_Request, opts ...grpc.CallOption) (*ServerAccessIP_Response, error)
	ServerAccessCountry(ctx context.Context, in *ServerAccessCountry_Request, opts ...grpc.CallOption) (*ServerAccessCountry_Response, error)
}

type firewallHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewFirewallHandlersClient(cc grpc.ClientConnInterface) FirewallHandlersClient {
	return &firewallHandlersClient{cc}
}

func (c *firewallHandlersClient) IPAccess(ctx context.Context, in *IPAccess_Request, opts ...grpc.CallOption) (*IPAccess_Response, error) {
	out := new(IPAccess_Response)
	err := c.cc.Invoke(ctx, FirewallHandlers_IPAccess_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *firewallHandlersClient) ServerFirewall(ctx context.Context, in *ServerFirewall_Request, opts ...grpc.CallOption) (*ServerFirewall_Response, error) {
	out := new(ServerFirewall_Response)
	err := c.cc.Invoke(ctx, FirewallHandlers_ServerFirewall_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *firewallHandlersClient) AddServerFirewall(ctx context.Context, in *AddServerFirewall_Request, opts ...grpc.CallOption) (*AddServerFirewall_Response, error) {
	out := new(AddServerFirewall_Response)
	err := c.cc.Invoke(ctx, FirewallHandlers_AddServerFirewall_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *firewallHandlersClient) UpdateServerFirewall(ctx context.Context, in *UpdateServerFirewall_Request, opts ...grpc.CallOption) (*UpdateServerFirewall_Response, error) {
	out := new(UpdateServerFirewall_Response)
	err := c.cc.Invoke(ctx, FirewallHandlers_UpdateServerFirewall_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *firewallHandlersClient) DeleteServerFirewall(ctx context.Context, in *DeleteServerFirewall_Request, opts ...grpc.CallOption) (*DeleteServerFirewall_Response, error) {
	out := new(DeleteServerFirewall_Response)
	err := c.cc.Invoke(ctx, FirewallHandlers_DeleteServerFirewall_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *firewallHandlersClient) ServerAccess(ctx context.Context, in *ServerAccess_Request, opts ...grpc.CallOption) (*ServerAccess_Response, error) {
	out := new(ServerAccess_Response)
	err := c.cc.Invoke(ctx, FirewallHandlers_ServerAccess_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *firewallHandlersClient) ServerAccessUser(ctx context.Context, in *ServerAccessUser_Request, opts ...grpc.CallOption) (*ServerAccessUser_Response, error) {
	out := new(ServerAccessUser_Response)
	err := c.cc.Invoke(ctx, FirewallHandlers_ServerAccessUser_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *firewallHandlersClient) ServerAccessTime(ctx context.Context, in *ServerAccessTime_Request, opts ...grpc.CallOption) (*ServerAccessTime_Response, error) {
	out := new(ServerAccessTime_Response)
	err := c.cc.Invoke(ctx, FirewallHandlers_ServerAccessTime_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *firewallHandlersClient) ServerAccessIP(ctx context.Context, in *ServerAccessIP_Request, opts ...grpc.CallOption) (*ServerAccessIP_Response, error) {
	out := new(ServerAccessIP_Response)
	err := c.cc.Invoke(ctx, FirewallHandlers_ServerAccessIP_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *firewallHandlersClient) ServerAccessCountry(ctx context.Context, in *ServerAccessCountry_Request, opts ...grpc.CallOption) (*ServerAccessCountry_Response, error) {
	out := new(ServerAccessCountry_Response)
	err := c.cc.Invoke(ctx, FirewallHandlers_ServerAccessCountry_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FirewallHandlersServer is the server API for FirewallHandlers service.
// All implementations must embed UnimplementedFirewallHandlersServer
// for forward compatibility
type FirewallHandlersServer interface {
	IPAccess(context.Context, *IPAccess_Request) (*IPAccess_Response, error)
	// Server firewall section
	ServerFirewall(context.Context, *ServerFirewall_Request) (*ServerFirewall_Response, error)
	AddServerFirewall(context.Context, *AddServerFirewall_Request) (*AddServerFirewall_Response, error)
	UpdateServerFirewall(context.Context, *UpdateServerFirewall_Request) (*UpdateServerFirewall_Response, error)
	DeleteServerFirewall(context.Context, *DeleteServerFirewall_Request) (*DeleteServerFirewall_Response, error)
	// Server access section
	ServerAccess(context.Context, *ServerAccess_Request) (*ServerAccess_Response, error)
	ServerAccessUser(context.Context, *ServerAccessUser_Request) (*ServerAccessUser_Response, error)
	ServerAccessTime(context.Context, *ServerAccessTime_Request) (*ServerAccessTime_Response, error)
	ServerAccessIP(context.Context, *ServerAccessIP_Request) (*ServerAccessIP_Response, error)
	ServerAccessCountry(context.Context, *ServerAccessCountry_Request) (*ServerAccessCountry_Response, error)
	mustEmbedUnimplementedFirewallHandlersServer()
}

// UnimplementedFirewallHandlersServer must be embedded to have forward compatible implementations.
type UnimplementedFirewallHandlersServer struct {
}

func (UnimplementedFirewallHandlersServer) IPAccess(context.Context, *IPAccess_Request) (*IPAccess_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method IPAccess not implemented")
}
func (UnimplementedFirewallHandlersServer) ServerFirewall(context.Context, *ServerFirewall_Request) (*ServerFirewall_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerFirewall not implemented")
}
func (UnimplementedFirewallHandlersServer) AddServerFirewall(context.Context, *AddServerFirewall_Request) (*AddServerFirewall_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddServerFirewall not implemented")
}
func (UnimplementedFirewallHandlersServer) UpdateServerFirewall(context.Context, *UpdateServerFirewall_Request) (*UpdateServerFirewall_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServerFirewall not implemented")
}
func (UnimplementedFirewallHandlersServer) DeleteServerFirewall(context.Context, *DeleteServerFirewall_Request) (*DeleteServerFirewall_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteServerFirewall not implemented")
}
func (UnimplementedFirewallHandlersServer) ServerAccess(context.Context, *ServerAccess_Request) (*ServerAccess_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerAccess not implemented")
}
func (UnimplementedFirewallHandlersServer) ServerAccessUser(context.Context, *ServerAccessUser_Request) (*ServerAccessUser_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerAccessUser not implemented")
}
func (UnimplementedFirewallHandlersServer) ServerAccessTime(context.Context, *ServerAccessTime_Request) (*ServerAccessTime_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerAccessTime not implemented")
}
func (UnimplementedFirewallHandlersServer) ServerAccessIP(context.Context, *ServerAccessIP_Request) (*ServerAccessIP_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerAccessIP not implemented")
}
func (UnimplementedFirewallHandlersServer) ServerAccessCountry(context.Context, *ServerAccessCountry_Request) (*ServerAccessCountry_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerAccessCountry not implemented")
}
func (UnimplementedFirewallHandlersServer) mustEmbedUnimplementedFirewallHandlersServer() {}

// UnsafeFirewallHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to FirewallHandlersServer will
// result in compilation errors.
type UnsafeFirewallHandlersServer interface {
	mustEmbedUnimplementedFirewallHandlersServer()
}

func RegisterFirewallHandlersServer(s grpc.ServiceRegistrar, srv FirewallHandlersServer) {
	s.RegisterService(&FirewallHandlers_ServiceDesc, srv)
}

func _FirewallHandlers_IPAccess_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(IPAccess_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FirewallHandlersServer).IPAccess(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FirewallHandlers_IPAccess_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FirewallHandlersServer).IPAccess(ctx, req.(*IPAccess_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _FirewallHandlers_ServerFirewall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerFirewall_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FirewallHandlersServer).ServerFirewall(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FirewallHandlers_ServerFirewall_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FirewallHandlersServer).ServerFirewall(ctx, req.(*ServerFirewall_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _FirewallHandlers_AddServerFirewall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddServerFirewall_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FirewallHandlersServer).AddServerFirewall(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FirewallHandlers_AddServerFirewall_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FirewallHandlersServer).AddServerFirewall(ctx, req.(*AddServerFirewall_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _FirewallHandlers_UpdateServerFirewall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateServerFirewall_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FirewallHandlersServer).UpdateServerFirewall(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FirewallHandlers_UpdateServerFirewall_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FirewallHandlersServer).UpdateServerFirewall(ctx, req.(*UpdateServerFirewall_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _FirewallHandlers_DeleteServerFirewall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteServerFirewall_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FirewallHandlersServer).DeleteServerFirewall(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FirewallHandlers_DeleteServerFirewall_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FirewallHandlersServer).DeleteServerFirewall(ctx, req.(*DeleteServerFirewall_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _FirewallHandlers_ServerAccess_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerAccess_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FirewallHandlersServer).ServerAccess(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FirewallHandlers_ServerAccess_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FirewallHandlersServer).ServerAccess(ctx, req.(*ServerAccess_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _FirewallHandlers_ServerAccessUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerAccessUser_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FirewallHandlersServer).ServerAccessUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FirewallHandlers_ServerAccessUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FirewallHandlersServer).ServerAccessUser(ctx, req.(*ServerAccessUser_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _FirewallHandlers_ServerAccessTime_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerAccessTime_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FirewallHandlersServer).ServerAccessTime(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FirewallHandlers_ServerAccessTime_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FirewallHandlersServer).ServerAccessTime(ctx, req.(*ServerAccessTime_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _FirewallHandlers_ServerAccessIP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerAccessIP_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FirewallHandlersServer).ServerAccessIP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FirewallHandlers_ServerAccessIP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FirewallHandlersServer).ServerAccessIP(ctx, req.(*ServerAccessIP_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _FirewallHandlers_ServerAccessCountry_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerAccessCountry_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FirewallHandlersServer).ServerAccessCountry(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FirewallHandlers_ServerAccessCountry_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FirewallHandlersServer).ServerAccessCountry(ctx, req.(*ServerAccessCountry_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// FirewallHandlers_ServiceDesc is the grpc.ServiceDesc for FirewallHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var FirewallHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "firewall.FirewallHandlers",
	HandlerType: (*FirewallHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "IPAccess",
			Handler:    _FirewallHandlers_IPAccess_Handler,
		},
		{
			MethodName: "ServerFirewall",
			Handler:    _FirewallHandlers_ServerFirewall_Handler,
		},
		{
			MethodName: "AddServerFirewall",
			Handler:    _FirewallHandlers_AddServerFirewall_Handler,
		},
		{
			MethodName: "UpdateServerFirewall",
			Handler:    _FirewallHandlers_UpdateServerFirewall_Handler,
		},
		{
			MethodName: "DeleteServerFirewall",
			Handler:    _FirewallHandlers_DeleteServerFirewall_Handler,
		},
		{
			MethodName: "ServerAccess",
			Handler:    _FirewallHandlers_ServerAccess_Handler,
		},
		{
			MethodName: "ServerAccessUser",
			Handler:    _FirewallHandlers_ServerAccessUser_Handler,
		},
		{
			MethodName: "ServerAccessTime",
			Handler:    _FirewallHandlers_ServerAccessTime_Handler,
		},
		{
			MethodName: "ServerAccessIP",
			Handler:    _FirewallHandlers_ServerAccessIP_Handler,
		},
		{
			MethodName: "ServerAccessCountry",
			Handler:    _FirewallHandlers_ServerAccessCountry_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "firewall.proto",
}
