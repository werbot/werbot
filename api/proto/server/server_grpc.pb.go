// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.7
// source: server.proto

package server

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

// ServerHandlersClient is the client API for ServerHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ServerHandlersClient interface {
	ListServers(ctx context.Context, in *ListServers_Request, opts ...grpc.CallOption) (*ListServers_Response, error)
	Server(ctx context.Context, in *Server_Request, opts ...grpc.CallOption) (*Server_Response, error)
	AddServer(ctx context.Context, in *AddServer_Request, opts ...grpc.CallOption) (*AddServer_Response, error)
	UpdateServer(ctx context.Context, in *UpdateServer_Request, opts ...grpc.CallOption) (*UpdateServer_Response, error)
	DeleteServer(ctx context.Context, in *DeleteServer_Request, opts ...grpc.CallOption) (*DeleteServer_Response, error)
	// TODO: replace to UpdateServerStatus
	UpdateServerOnlineStatus(ctx context.Context, in *UpdateServerOnlineStatus_Request, opts ...grpc.CallOption) (*UpdateServerOnlineStatus_Response, error)
	UpdateServerActiveStatus(ctx context.Context, in *UpdateServerActiveStatus_Request, opts ...grpc.CallOption) (*UpdateServerActiveStatus_Response, error)
	ServerAccess(ctx context.Context, in *ServerAccess_Request, opts ...grpc.CallOption) (*ServerAccess_Response, error)
	UpdateServerAccess(ctx context.Context, in *UpdateServerAccess_Request, opts ...grpc.CallOption) (*UpdateServerAccess_Response, error)
	ServerActivity(ctx context.Context, in *ServerActivity_Request, opts ...grpc.CallOption) (*ServerActivity_Response, error)
	UpdateServerActivity(ctx context.Context, in *UpdateServerActivity_Request, opts ...grpc.CallOption) (*UpdateServerActivity_Response, error)
	UpdateServerHostKey(ctx context.Context, in *UpdateServerHostKey_Request, opts ...grpc.CallOption) (*UpdateServerHostKey_Response, error)
	AddServerSession(ctx context.Context, in *AddServerSession_Request, opts ...grpc.CallOption) (*AddServerSession_Response, error)
	ServerNameByID(ctx context.Context, in *ServerNameByID_Request, opts ...grpc.CallOption) (*ServerNameByID_Response, error)
	// share server
	ListServersShareForUser(ctx context.Context, in *ListServersShareForUser_Request, opts ...grpc.CallOption) (*ListServersShareForUser_Response, error)
	AddServerShareForUser(ctx context.Context, in *AddServerShareForUser_Request, opts ...grpc.CallOption) (*AddServerShareForUser_Response, error)
	UpdateServerShareForUser(ctx context.Context, in *UpdateServerShareForUser_Request, opts ...grpc.CallOption) (*UpdateServerShareForUser_Response, error)
	DeleteServerShareForUser(ctx context.Context, in *DeleteServerShareForUser_Request, opts ...grpc.CallOption) (*DeleteServerShareForUser_Response, error)
}

type serverHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewServerHandlersClient(cc grpc.ClientConnInterface) ServerHandlersClient {
	return &serverHandlersClient{cc}
}

func (c *serverHandlersClient) ListServers(ctx context.Context, in *ListServers_Request, opts ...grpc.CallOption) (*ListServers_Response, error) {
	out := new(ListServers_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/ListServers", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) Server(ctx context.Context, in *Server_Request, opts ...grpc.CallOption) (*Server_Response, error) {
	out := new(Server_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/Server", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) AddServer(ctx context.Context, in *AddServer_Request, opts ...grpc.CallOption) (*AddServer_Response, error) {
	out := new(AddServer_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/AddServer", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateServer(ctx context.Context, in *UpdateServer_Request, opts ...grpc.CallOption) (*UpdateServer_Response, error) {
	out := new(UpdateServer_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/UpdateServer", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) DeleteServer(ctx context.Context, in *DeleteServer_Request, opts ...grpc.CallOption) (*DeleteServer_Response, error) {
	out := new(DeleteServer_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/DeleteServer", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateServerOnlineStatus(ctx context.Context, in *UpdateServerOnlineStatus_Request, opts ...grpc.CallOption) (*UpdateServerOnlineStatus_Response, error) {
	out := new(UpdateServerOnlineStatus_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/UpdateServerOnlineStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateServerActiveStatus(ctx context.Context, in *UpdateServerActiveStatus_Request, opts ...grpc.CallOption) (*UpdateServerActiveStatus_Response, error) {
	out := new(UpdateServerActiveStatus_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/UpdateServerActiveStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) ServerAccess(ctx context.Context, in *ServerAccess_Request, opts ...grpc.CallOption) (*ServerAccess_Response, error) {
	out := new(ServerAccess_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/ServerAccess", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateServerAccess(ctx context.Context, in *UpdateServerAccess_Request, opts ...grpc.CallOption) (*UpdateServerAccess_Response, error) {
	out := new(UpdateServerAccess_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/UpdateServerAccess", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) ServerActivity(ctx context.Context, in *ServerActivity_Request, opts ...grpc.CallOption) (*ServerActivity_Response, error) {
	out := new(ServerActivity_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/ServerActivity", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateServerActivity(ctx context.Context, in *UpdateServerActivity_Request, opts ...grpc.CallOption) (*UpdateServerActivity_Response, error) {
	out := new(UpdateServerActivity_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/UpdateServerActivity", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateServerHostKey(ctx context.Context, in *UpdateServerHostKey_Request, opts ...grpc.CallOption) (*UpdateServerHostKey_Response, error) {
	out := new(UpdateServerHostKey_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/UpdateServerHostKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) AddServerSession(ctx context.Context, in *AddServerSession_Request, opts ...grpc.CallOption) (*AddServerSession_Response, error) {
	out := new(AddServerSession_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/AddServerSession", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) ServerNameByID(ctx context.Context, in *ServerNameByID_Request, opts ...grpc.CallOption) (*ServerNameByID_Response, error) {
	out := new(ServerNameByID_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/ServerNameByID", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) ListServersShareForUser(ctx context.Context, in *ListServersShareForUser_Request, opts ...grpc.CallOption) (*ListServersShareForUser_Response, error) {
	out := new(ListServersShareForUser_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/ListServersShareForUser", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) AddServerShareForUser(ctx context.Context, in *AddServerShareForUser_Request, opts ...grpc.CallOption) (*AddServerShareForUser_Response, error) {
	out := new(AddServerShareForUser_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/AddServerShareForUser", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateServerShareForUser(ctx context.Context, in *UpdateServerShareForUser_Request, opts ...grpc.CallOption) (*UpdateServerShareForUser_Response, error) {
	out := new(UpdateServerShareForUser_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/UpdateServerShareForUser", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) DeleteServerShareForUser(ctx context.Context, in *DeleteServerShareForUser_Request, opts ...grpc.CallOption) (*DeleteServerShareForUser_Response, error) {
	out := new(DeleteServerShareForUser_Response)
	err := c.cc.Invoke(ctx, "/server.ServerHandlers/DeleteServerShareForUser", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ServerHandlersServer is the server API for ServerHandlers service.
// All implementations must embed UnimplementedServerHandlersServer
// for forward compatibility
type ServerHandlersServer interface {
	ListServers(context.Context, *ListServers_Request) (*ListServers_Response, error)
	Server(context.Context, *Server_Request) (*Server_Response, error)
	AddServer(context.Context, *AddServer_Request) (*AddServer_Response, error)
	UpdateServer(context.Context, *UpdateServer_Request) (*UpdateServer_Response, error)
	DeleteServer(context.Context, *DeleteServer_Request) (*DeleteServer_Response, error)
	// TODO: replace to UpdateServerStatus
	UpdateServerOnlineStatus(context.Context, *UpdateServerOnlineStatus_Request) (*UpdateServerOnlineStatus_Response, error)
	UpdateServerActiveStatus(context.Context, *UpdateServerActiveStatus_Request) (*UpdateServerActiveStatus_Response, error)
	ServerAccess(context.Context, *ServerAccess_Request) (*ServerAccess_Response, error)
	UpdateServerAccess(context.Context, *UpdateServerAccess_Request) (*UpdateServerAccess_Response, error)
	ServerActivity(context.Context, *ServerActivity_Request) (*ServerActivity_Response, error)
	UpdateServerActivity(context.Context, *UpdateServerActivity_Request) (*UpdateServerActivity_Response, error)
	UpdateServerHostKey(context.Context, *UpdateServerHostKey_Request) (*UpdateServerHostKey_Response, error)
	AddServerSession(context.Context, *AddServerSession_Request) (*AddServerSession_Response, error)
	ServerNameByID(context.Context, *ServerNameByID_Request) (*ServerNameByID_Response, error)
	// share server
	ListServersShareForUser(context.Context, *ListServersShareForUser_Request) (*ListServersShareForUser_Response, error)
	AddServerShareForUser(context.Context, *AddServerShareForUser_Request) (*AddServerShareForUser_Response, error)
	UpdateServerShareForUser(context.Context, *UpdateServerShareForUser_Request) (*UpdateServerShareForUser_Response, error)
	DeleteServerShareForUser(context.Context, *DeleteServerShareForUser_Request) (*DeleteServerShareForUser_Response, error)
	mustEmbedUnimplementedServerHandlersServer()
}

// UnimplementedServerHandlersServer must be embedded to have forward compatible implementations.
type UnimplementedServerHandlersServer struct {
}

func (UnimplementedServerHandlersServer) ListServers(context.Context, *ListServers_Request) (*ListServers_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListServers not implemented")
}
func (UnimplementedServerHandlersServer) Server(context.Context, *Server_Request) (*Server_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Server not implemented")
}
func (UnimplementedServerHandlersServer) AddServer(context.Context, *AddServer_Request) (*AddServer_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddServer not implemented")
}
func (UnimplementedServerHandlersServer) UpdateServer(context.Context, *UpdateServer_Request) (*UpdateServer_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServer not implemented")
}
func (UnimplementedServerHandlersServer) DeleteServer(context.Context, *DeleteServer_Request) (*DeleteServer_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteServer not implemented")
}
func (UnimplementedServerHandlersServer) UpdateServerOnlineStatus(context.Context, *UpdateServerOnlineStatus_Request) (*UpdateServerOnlineStatus_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServerOnlineStatus not implemented")
}
func (UnimplementedServerHandlersServer) UpdateServerActiveStatus(context.Context, *UpdateServerActiveStatus_Request) (*UpdateServerActiveStatus_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServerActiveStatus not implemented")
}
func (UnimplementedServerHandlersServer) ServerAccess(context.Context, *ServerAccess_Request) (*ServerAccess_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerAccess not implemented")
}
func (UnimplementedServerHandlersServer) UpdateServerAccess(context.Context, *UpdateServerAccess_Request) (*UpdateServerAccess_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServerAccess not implemented")
}
func (UnimplementedServerHandlersServer) ServerActivity(context.Context, *ServerActivity_Request) (*ServerActivity_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerActivity not implemented")
}
func (UnimplementedServerHandlersServer) UpdateServerActivity(context.Context, *UpdateServerActivity_Request) (*UpdateServerActivity_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServerActivity not implemented")
}
func (UnimplementedServerHandlersServer) UpdateServerHostKey(context.Context, *UpdateServerHostKey_Request) (*UpdateServerHostKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServerHostKey not implemented")
}
func (UnimplementedServerHandlersServer) AddServerSession(context.Context, *AddServerSession_Request) (*AddServerSession_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddServerSession not implemented")
}
func (UnimplementedServerHandlersServer) ServerNameByID(context.Context, *ServerNameByID_Request) (*ServerNameByID_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerNameByID not implemented")
}
func (UnimplementedServerHandlersServer) ListServersShareForUser(context.Context, *ListServersShareForUser_Request) (*ListServersShareForUser_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListServersShareForUser not implemented")
}
func (UnimplementedServerHandlersServer) AddServerShareForUser(context.Context, *AddServerShareForUser_Request) (*AddServerShareForUser_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddServerShareForUser not implemented")
}
func (UnimplementedServerHandlersServer) UpdateServerShareForUser(context.Context, *UpdateServerShareForUser_Request) (*UpdateServerShareForUser_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServerShareForUser not implemented")
}
func (UnimplementedServerHandlersServer) DeleteServerShareForUser(context.Context, *DeleteServerShareForUser_Request) (*DeleteServerShareForUser_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteServerShareForUser not implemented")
}
func (UnimplementedServerHandlersServer) mustEmbedUnimplementedServerHandlersServer() {}

// UnsafeServerHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ServerHandlersServer will
// result in compilation errors.
type UnsafeServerHandlersServer interface {
	mustEmbedUnimplementedServerHandlersServer()
}

func RegisterServerHandlersServer(s grpc.ServiceRegistrar, srv ServerHandlersServer) {
	s.RegisterService(&ServerHandlers_ServiceDesc, srv)
}

func _ServerHandlers_ListServers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListServers_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).ListServers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/ListServers",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).ListServers(ctx, req.(*ListServers_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_Server_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Server_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).Server(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/Server",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).Server(ctx, req.(*Server_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_AddServer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddServer_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).AddServer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/AddServer",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).AddServer(ctx, req.(*AddServer_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_UpdateServer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateServer_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).UpdateServer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/UpdateServer",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).UpdateServer(ctx, req.(*UpdateServer_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_DeleteServer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteServer_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).DeleteServer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/DeleteServer",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).DeleteServer(ctx, req.(*DeleteServer_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_UpdateServerOnlineStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateServerOnlineStatus_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).UpdateServerOnlineStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/UpdateServerOnlineStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).UpdateServerOnlineStatus(ctx, req.(*UpdateServerOnlineStatus_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_UpdateServerActiveStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateServerActiveStatus_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).UpdateServerActiveStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/UpdateServerActiveStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).UpdateServerActiveStatus(ctx, req.(*UpdateServerActiveStatus_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_ServerAccess_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerAccess_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).ServerAccess(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/ServerAccess",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).ServerAccess(ctx, req.(*ServerAccess_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_UpdateServerAccess_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateServerAccess_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).UpdateServerAccess(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/UpdateServerAccess",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).UpdateServerAccess(ctx, req.(*UpdateServerAccess_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_ServerActivity_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerActivity_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).ServerActivity(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/ServerActivity",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).ServerActivity(ctx, req.(*ServerActivity_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_UpdateServerActivity_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateServerActivity_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).UpdateServerActivity(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/UpdateServerActivity",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).UpdateServerActivity(ctx, req.(*UpdateServerActivity_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_UpdateServerHostKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateServerHostKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).UpdateServerHostKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/UpdateServerHostKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).UpdateServerHostKey(ctx, req.(*UpdateServerHostKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_AddServerSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddServerSession_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).AddServerSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/AddServerSession",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).AddServerSession(ctx, req.(*AddServerSession_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_ServerNameByID_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerNameByID_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).ServerNameByID(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/ServerNameByID",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).ServerNameByID(ctx, req.(*ServerNameByID_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_ListServersShareForUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListServersShareForUser_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).ListServersShareForUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/ListServersShareForUser",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).ListServersShareForUser(ctx, req.(*ListServersShareForUser_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_AddServerShareForUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddServerShareForUser_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).AddServerShareForUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/AddServerShareForUser",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).AddServerShareForUser(ctx, req.(*AddServerShareForUser_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_UpdateServerShareForUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateServerShareForUser_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).UpdateServerShareForUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/UpdateServerShareForUser",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).UpdateServerShareForUser(ctx, req.(*UpdateServerShareForUser_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_DeleteServerShareForUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteServerShareForUser_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).DeleteServerShareForUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/server.ServerHandlers/DeleteServerShareForUser",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).DeleteServerShareForUser(ctx, req.(*DeleteServerShareForUser_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// ServerHandlers_ServiceDesc is the grpc.ServiceDesc for ServerHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ServerHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "server.ServerHandlers",
	HandlerType: (*ServerHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListServers",
			Handler:    _ServerHandlers_ListServers_Handler,
		},
		{
			MethodName: "Server",
			Handler:    _ServerHandlers_Server_Handler,
		},
		{
			MethodName: "AddServer",
			Handler:    _ServerHandlers_AddServer_Handler,
		},
		{
			MethodName: "UpdateServer",
			Handler:    _ServerHandlers_UpdateServer_Handler,
		},
		{
			MethodName: "DeleteServer",
			Handler:    _ServerHandlers_DeleteServer_Handler,
		},
		{
			MethodName: "UpdateServerOnlineStatus",
			Handler:    _ServerHandlers_UpdateServerOnlineStatus_Handler,
		},
		{
			MethodName: "UpdateServerActiveStatus",
			Handler:    _ServerHandlers_UpdateServerActiveStatus_Handler,
		},
		{
			MethodName: "ServerAccess",
			Handler:    _ServerHandlers_ServerAccess_Handler,
		},
		{
			MethodName: "UpdateServerAccess",
			Handler:    _ServerHandlers_UpdateServerAccess_Handler,
		},
		{
			MethodName: "ServerActivity",
			Handler:    _ServerHandlers_ServerActivity_Handler,
		},
		{
			MethodName: "UpdateServerActivity",
			Handler:    _ServerHandlers_UpdateServerActivity_Handler,
		},
		{
			MethodName: "UpdateServerHostKey",
			Handler:    _ServerHandlers_UpdateServerHostKey_Handler,
		},
		{
			MethodName: "AddServerSession",
			Handler:    _ServerHandlers_AddServerSession_Handler,
		},
		{
			MethodName: "ServerNameByID",
			Handler:    _ServerHandlers_ServerNameByID_Handler,
		},
		{
			MethodName: "ListServersShareForUser",
			Handler:    _ServerHandlers_ListServersShareForUser_Handler,
		},
		{
			MethodName: "AddServerShareForUser",
			Handler:    _ServerHandlers_AddServerShareForUser_Handler,
		},
		{
			MethodName: "UpdateServerShareForUser",
			Handler:    _ServerHandlers_UpdateServerShareForUser_Handler,
		},
		{
			MethodName: "DeleteServerShareForUser",
			Handler:    _ServerHandlers_DeleteServerShareForUser_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "server.proto",
}
