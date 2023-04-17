// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.22.3
// source: server.proto

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
	ServerHandlers_ListServers_FullMethodName          = "/server.ServerHandlers/ListServers"
	ServerHandlers_Server_FullMethodName               = "/server.ServerHandlers/Server"
	ServerHandlers_AddServer_FullMethodName            = "/server.ServerHandlers/AddServer"
	ServerHandlers_UpdateServer_FullMethodName         = "/server.ServerHandlers/UpdateServer"
	ServerHandlers_DeleteServer_FullMethodName         = "/server.ServerHandlers/DeleteServer"
	ServerHandlers_ServerAccess_FullMethodName         = "/server.ServerHandlers/ServerAccess"
	ServerHandlers_UpdateServerAccess_FullMethodName   = "/server.ServerHandlers/UpdateServerAccess"
	ServerHandlers_ServerActivity_FullMethodName       = "/server.ServerHandlers/ServerActivity"
	ServerHandlers_UpdateServerActivity_FullMethodName = "/server.ServerHandlers/UpdateServerActivity"
	ServerHandlers_ListShareServers_FullMethodName     = "/server.ServerHandlers/ListShareServers"
	ServerHandlers_AddShareServer_FullMethodName       = "/server.ServerHandlers/AddShareServer"
	ServerHandlers_UpdateShareServer_FullMethodName    = "/server.ServerHandlers/UpdateShareServer"
	ServerHandlers_DeleteShareServer_FullMethodName    = "/server.ServerHandlers/DeleteShareServer"
	ServerHandlers_UpdateHostKey_FullMethodName        = "/server.ServerHandlers/UpdateHostKey"
	ServerHandlers_AddSession_FullMethodName           = "/server.ServerHandlers/AddSession"
	ServerHandlers_ServerNameByID_FullMethodName       = "/server.ServerHandlers/ServerNameByID"
)

// ServerHandlersClient is the client API for ServerHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ServerHandlersClient interface {
	// Server section
	ListServers(ctx context.Context, in *ListServers_Request, opts ...grpc.CallOption) (*ListServers_Response, error)
	Server(ctx context.Context, in *Server_Request, opts ...grpc.CallOption) (*Server_Response, error)
	AddServer(ctx context.Context, in *AddServer_Request, opts ...grpc.CallOption) (*AddServer_Response, error)
	UpdateServer(ctx context.Context, in *UpdateServer_Request, opts ...grpc.CallOption) (*UpdateServer_Response, error)
	DeleteServer(ctx context.Context, in *DeleteServer_Request, opts ...grpc.CallOption) (*DeleteServer_Response, error)
	// Server Access section
	ServerAccess(ctx context.Context, in *ServerAccess_Request, opts ...grpc.CallOption) (*ServerAccess_Response, error)
	UpdateServerAccess(ctx context.Context, in *UpdateServerAccess_Request, opts ...grpc.CallOption) (*UpdateServerAccess_Response, error)
	// Server Activity section
	ServerActivity(ctx context.Context, in *ServerActivity_Request, opts ...grpc.CallOption) (*ServerActivity_Response, error)
	UpdateServerActivity(ctx context.Context, in *UpdateServerActivity_Request, opts ...grpc.CallOption) (*UpdateServerActivity_Response, error)
	// share server
	ListShareServers(ctx context.Context, in *ListShareServers_Request, opts ...grpc.CallOption) (*ListShareServers_Response, error)
	AddShareServer(ctx context.Context, in *AddShareServer_Request, opts ...grpc.CallOption) (*AddShareServer_Response, error)
	UpdateShareServer(ctx context.Context, in *UpdateShareServer_Request, opts ...grpc.CallOption) (*UpdateShareServer_Response, error)
	DeleteShareServer(ctx context.Context, in *DeleteShareServer_Request, opts ...grpc.CallOption) (*DeleteShareServer_Response, error)
	// Other
	UpdateHostKey(ctx context.Context, in *UpdateHostKey_Request, opts ...grpc.CallOption) (*UpdateHostKey_Response, error)
	AddSession(ctx context.Context, in *AddSession_Request, opts ...grpc.CallOption) (*AddSession_Response, error)
	ServerNameByID(ctx context.Context, in *ServerNameByID_Request, opts ...grpc.CallOption) (*ServerNameByID_Response, error)
}

type serverHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewServerHandlersClient(cc grpc.ClientConnInterface) ServerHandlersClient {
	return &serverHandlersClient{cc}
}

func (c *serverHandlersClient) ListServers(ctx context.Context, in *ListServers_Request, opts ...grpc.CallOption) (*ListServers_Response, error) {
	out := new(ListServers_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_ListServers_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) Server(ctx context.Context, in *Server_Request, opts ...grpc.CallOption) (*Server_Response, error) {
	out := new(Server_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_Server_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) AddServer(ctx context.Context, in *AddServer_Request, opts ...grpc.CallOption) (*AddServer_Response, error) {
	out := new(AddServer_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_AddServer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateServer(ctx context.Context, in *UpdateServer_Request, opts ...grpc.CallOption) (*UpdateServer_Response, error) {
	out := new(UpdateServer_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_UpdateServer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) DeleteServer(ctx context.Context, in *DeleteServer_Request, opts ...grpc.CallOption) (*DeleteServer_Response, error) {
	out := new(DeleteServer_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_DeleteServer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) ServerAccess(ctx context.Context, in *ServerAccess_Request, opts ...grpc.CallOption) (*ServerAccess_Response, error) {
	out := new(ServerAccess_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_ServerAccess_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateServerAccess(ctx context.Context, in *UpdateServerAccess_Request, opts ...grpc.CallOption) (*UpdateServerAccess_Response, error) {
	out := new(UpdateServerAccess_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_UpdateServerAccess_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) ServerActivity(ctx context.Context, in *ServerActivity_Request, opts ...grpc.CallOption) (*ServerActivity_Response, error) {
	out := new(ServerActivity_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_ServerActivity_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateServerActivity(ctx context.Context, in *UpdateServerActivity_Request, opts ...grpc.CallOption) (*UpdateServerActivity_Response, error) {
	out := new(UpdateServerActivity_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_UpdateServerActivity_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) ListShareServers(ctx context.Context, in *ListShareServers_Request, opts ...grpc.CallOption) (*ListShareServers_Response, error) {
	out := new(ListShareServers_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_ListShareServers_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) AddShareServer(ctx context.Context, in *AddShareServer_Request, opts ...grpc.CallOption) (*AddShareServer_Response, error) {
	out := new(AddShareServer_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_AddShareServer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateShareServer(ctx context.Context, in *UpdateShareServer_Request, opts ...grpc.CallOption) (*UpdateShareServer_Response, error) {
	out := new(UpdateShareServer_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_UpdateShareServer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) DeleteShareServer(ctx context.Context, in *DeleteShareServer_Request, opts ...grpc.CallOption) (*DeleteShareServer_Response, error) {
	out := new(DeleteShareServer_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_DeleteShareServer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) UpdateHostKey(ctx context.Context, in *UpdateHostKey_Request, opts ...grpc.CallOption) (*UpdateHostKey_Response, error) {
	out := new(UpdateHostKey_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_UpdateHostKey_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) AddSession(ctx context.Context, in *AddSession_Request, opts ...grpc.CallOption) (*AddSession_Response, error) {
	out := new(AddSession_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_AddSession_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serverHandlersClient) ServerNameByID(ctx context.Context, in *ServerNameByID_Request, opts ...grpc.CallOption) (*ServerNameByID_Response, error) {
	out := new(ServerNameByID_Response)
	err := c.cc.Invoke(ctx, ServerHandlers_ServerNameByID_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ServerHandlersServer is the server API for ServerHandlers service.
// All implementations must embed UnimplementedServerHandlersServer
// for forward compatibility
type ServerHandlersServer interface {
	// Server section
	ListServers(context.Context, *ListServers_Request) (*ListServers_Response, error)
	Server(context.Context, *Server_Request) (*Server_Response, error)
	AddServer(context.Context, *AddServer_Request) (*AddServer_Response, error)
	UpdateServer(context.Context, *UpdateServer_Request) (*UpdateServer_Response, error)
	DeleteServer(context.Context, *DeleteServer_Request) (*DeleteServer_Response, error)
	// Server Access section
	ServerAccess(context.Context, *ServerAccess_Request) (*ServerAccess_Response, error)
	UpdateServerAccess(context.Context, *UpdateServerAccess_Request) (*UpdateServerAccess_Response, error)
	// Server Activity section
	ServerActivity(context.Context, *ServerActivity_Request) (*ServerActivity_Response, error)
	UpdateServerActivity(context.Context, *UpdateServerActivity_Request) (*UpdateServerActivity_Response, error)
	// share server
	ListShareServers(context.Context, *ListShareServers_Request) (*ListShareServers_Response, error)
	AddShareServer(context.Context, *AddShareServer_Request) (*AddShareServer_Response, error)
	UpdateShareServer(context.Context, *UpdateShareServer_Request) (*UpdateShareServer_Response, error)
	DeleteShareServer(context.Context, *DeleteShareServer_Request) (*DeleteShareServer_Response, error)
	// Other
	UpdateHostKey(context.Context, *UpdateHostKey_Request) (*UpdateHostKey_Response, error)
	AddSession(context.Context, *AddSession_Request) (*AddSession_Response, error)
	ServerNameByID(context.Context, *ServerNameByID_Request) (*ServerNameByID_Response, error)
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
func (UnimplementedServerHandlersServer) ListShareServers(context.Context, *ListShareServers_Request) (*ListShareServers_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListShareServers not implemented")
}
func (UnimplementedServerHandlersServer) AddShareServer(context.Context, *AddShareServer_Request) (*AddShareServer_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddShareServer not implemented")
}
func (UnimplementedServerHandlersServer) UpdateShareServer(context.Context, *UpdateShareServer_Request) (*UpdateShareServer_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateShareServer not implemented")
}
func (UnimplementedServerHandlersServer) DeleteShareServer(context.Context, *DeleteShareServer_Request) (*DeleteShareServer_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteShareServer not implemented")
}
func (UnimplementedServerHandlersServer) UpdateHostKey(context.Context, *UpdateHostKey_Request) (*UpdateHostKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateHostKey not implemented")
}
func (UnimplementedServerHandlersServer) AddSession(context.Context, *AddSession_Request) (*AddSession_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddSession not implemented")
}
func (UnimplementedServerHandlersServer) ServerNameByID(context.Context, *ServerNameByID_Request) (*ServerNameByID_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerNameByID not implemented")
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
		FullMethod: ServerHandlers_ListServers_FullMethodName,
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
		FullMethod: ServerHandlers_Server_FullMethodName,
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
		FullMethod: ServerHandlers_AddServer_FullMethodName,
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
		FullMethod: ServerHandlers_UpdateServer_FullMethodName,
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
		FullMethod: ServerHandlers_DeleteServer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).DeleteServer(ctx, req.(*DeleteServer_Request))
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
		FullMethod: ServerHandlers_ServerAccess_FullMethodName,
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
		FullMethod: ServerHandlers_UpdateServerAccess_FullMethodName,
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
		FullMethod: ServerHandlers_ServerActivity_FullMethodName,
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
		FullMethod: ServerHandlers_UpdateServerActivity_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).UpdateServerActivity(ctx, req.(*UpdateServerActivity_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_ListShareServers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListShareServers_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).ListShareServers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServerHandlers_ListShareServers_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).ListShareServers(ctx, req.(*ListShareServers_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_AddShareServer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddShareServer_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).AddShareServer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServerHandlers_AddShareServer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).AddShareServer(ctx, req.(*AddShareServer_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_UpdateShareServer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateShareServer_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).UpdateShareServer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServerHandlers_UpdateShareServer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).UpdateShareServer(ctx, req.(*UpdateShareServer_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_DeleteShareServer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteShareServer_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).DeleteShareServer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServerHandlers_DeleteShareServer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).DeleteShareServer(ctx, req.(*DeleteShareServer_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_UpdateHostKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateHostKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).UpdateHostKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServerHandlers_UpdateHostKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).UpdateHostKey(ctx, req.(*UpdateHostKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServerHandlers_AddSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddSession_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerHandlersServer).AddSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServerHandlers_AddSession_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).AddSession(ctx, req.(*AddSession_Request))
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
		FullMethod: ServerHandlers_ServerNameByID_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerHandlersServer).ServerNameByID(ctx, req.(*ServerNameByID_Request))
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
			MethodName: "ListShareServers",
			Handler:    _ServerHandlers_ListShareServers_Handler,
		},
		{
			MethodName: "AddShareServer",
			Handler:    _ServerHandlers_AddShareServer_Handler,
		},
		{
			MethodName: "UpdateShareServer",
			Handler:    _ServerHandlers_UpdateShareServer_Handler,
		},
		{
			MethodName: "DeleteShareServer",
			Handler:    _ServerHandlers_DeleteShareServer_Handler,
		},
		{
			MethodName: "UpdateHostKey",
			Handler:    _ServerHandlers_UpdateHostKey_Handler,
		},
		{
			MethodName: "AddSession",
			Handler:    _ServerHandlers_AddSession_Handler,
		},
		{
			MethodName: "ServerNameByID",
			Handler:    _ServerHandlers_ServerNameByID_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "server.proto",
}
