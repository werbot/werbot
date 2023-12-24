// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.1
// source: member.proto

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
	MemberHandlers_ListProjectMembers_FullMethodName   = "/member.MemberHandlers/ListProjectMembers"
	MemberHandlers_ProjectMember_FullMethodName        = "/member.MemberHandlers/ProjectMember"
	MemberHandlers_AddProjectMember_FullMethodName     = "/member.MemberHandlers/AddProjectMember"
	MemberHandlers_UpdateProjectMember_FullMethodName  = "/member.MemberHandlers/UpdateProjectMember"
	MemberHandlers_DeleteProjectMember_FullMethodName  = "/member.MemberHandlers/DeleteProjectMember"
	MemberHandlers_UsersWithoutProject_FullMethodName  = "/member.MemberHandlers/UsersWithoutProject"
	MemberHandlers_ListMembersInvite_FullMethodName    = "/member.MemberHandlers/ListMembersInvite"
	MemberHandlers_AddMemberInvite_FullMethodName      = "/member.MemberHandlers/AddMemberInvite"
	MemberHandlers_DeleteMemberInvite_FullMethodName   = "/member.MemberHandlers/DeleteMemberInvite"
	MemberHandlers_MemberInviteActivate_FullMethodName = "/member.MemberHandlers/MemberInviteActivate"
	MemberHandlers_ListServerMembers_FullMethodName    = "/member.MemberHandlers/ListServerMembers"
	MemberHandlers_ServerMember_FullMethodName         = "/member.MemberHandlers/ServerMember"
	MemberHandlers_AddServerMember_FullMethodName      = "/member.MemberHandlers/AddServerMember"
	MemberHandlers_UpdateServerMember_FullMethodName   = "/member.MemberHandlers/UpdateServerMember"
	MemberHandlers_DeleteServerMember_FullMethodName   = "/member.MemberHandlers/DeleteServerMember"
	MemberHandlers_MembersWithoutServer_FullMethodName = "/member.MemberHandlers/MembersWithoutServer"
)

// MemberHandlersClient is the client API for MemberHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type MemberHandlersClient interface {
	// Project section
	ListProjectMembers(ctx context.Context, in *ListProjectMembers_Request, opts ...grpc.CallOption) (*ListProjectMembers_Response, error)
	ProjectMember(ctx context.Context, in *ProjectMember_Request, opts ...grpc.CallOption) (*ProjectMember_Response, error)
	AddProjectMember(ctx context.Context, in *AddProjectMember_Request, opts ...grpc.CallOption) (*AddProjectMember_Response, error)
	UpdateProjectMember(ctx context.Context, in *UpdateProjectMember_Request, opts ...grpc.CallOption) (*UpdateProjectMember_Response, error)
	DeleteProjectMember(ctx context.Context, in *DeleteProjectMember_Request, opts ...grpc.CallOption) (*DeleteProjectMember_Response, error)
	// Used in finding and adding a new member to the project
	UsersWithoutProject(ctx context.Context, in *UsersWithoutProject_Request, opts ...grpc.CallOption) (*UsersWithoutProject_Response, error)
	// Invite section
	ListMembersInvite(ctx context.Context, in *ListMembersInvite_Request, opts ...grpc.CallOption) (*ListMembersInvite_Response, error)
	AddMemberInvite(ctx context.Context, in *AddMemberInvite_Request, opts ...grpc.CallOption) (*AddMemberInvite_Response, error)
	DeleteMemberInvite(ctx context.Context, in *DeleteMemberInvite_Request, opts ...grpc.CallOption) (*DeleteMemberInvite_Response, error)
	MemberInviteActivate(ctx context.Context, in *MemberInviteActivate_Request, opts ...grpc.CallOption) (*MemberInviteActivate_Response, error)
	// Server section
	ListServerMembers(ctx context.Context, in *ListServerMembers_Request, opts ...grpc.CallOption) (*ListServerMembers_Response, error)
	ServerMember(ctx context.Context, in *ServerMember_Request, opts ...grpc.CallOption) (*ServerMember_Response, error)
	AddServerMember(ctx context.Context, in *AddServerMember_Request, opts ...grpc.CallOption) (*AddServerMember_Response, error)
	UpdateServerMember(ctx context.Context, in *UpdateServerMember_Request, opts ...grpc.CallOption) (*UpdateServerMember_Response, error)
	DeleteServerMember(ctx context.Context, in *DeleteServerMember_Request, opts ...grpc.CallOption) (*DeleteServerMember_Response, error)
	// Used in finding and adding a new member to the server
	MembersWithoutServer(ctx context.Context, in *MembersWithoutServer_Request, opts ...grpc.CallOption) (*MembersWithoutServer_Response, error)
}

type memberHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewMemberHandlersClient(cc grpc.ClientConnInterface) MemberHandlersClient {
	return &memberHandlersClient{cc}
}

func (c *memberHandlersClient) ListProjectMembers(ctx context.Context, in *ListProjectMembers_Request, opts ...grpc.CallOption) (*ListProjectMembers_Response, error) {
	out := new(ListProjectMembers_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_ListProjectMembers_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) ProjectMember(ctx context.Context, in *ProjectMember_Request, opts ...grpc.CallOption) (*ProjectMember_Response, error) {
	out := new(ProjectMember_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_ProjectMember_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) AddProjectMember(ctx context.Context, in *AddProjectMember_Request, opts ...grpc.CallOption) (*AddProjectMember_Response, error) {
	out := new(AddProjectMember_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_AddProjectMember_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) UpdateProjectMember(ctx context.Context, in *UpdateProjectMember_Request, opts ...grpc.CallOption) (*UpdateProjectMember_Response, error) {
	out := new(UpdateProjectMember_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_UpdateProjectMember_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) DeleteProjectMember(ctx context.Context, in *DeleteProjectMember_Request, opts ...grpc.CallOption) (*DeleteProjectMember_Response, error) {
	out := new(DeleteProjectMember_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_DeleteProjectMember_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) UsersWithoutProject(ctx context.Context, in *UsersWithoutProject_Request, opts ...grpc.CallOption) (*UsersWithoutProject_Response, error) {
	out := new(UsersWithoutProject_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_UsersWithoutProject_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) ListMembersInvite(ctx context.Context, in *ListMembersInvite_Request, opts ...grpc.CallOption) (*ListMembersInvite_Response, error) {
	out := new(ListMembersInvite_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_ListMembersInvite_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) AddMemberInvite(ctx context.Context, in *AddMemberInvite_Request, opts ...grpc.CallOption) (*AddMemberInvite_Response, error) {
	out := new(AddMemberInvite_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_AddMemberInvite_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) DeleteMemberInvite(ctx context.Context, in *DeleteMemberInvite_Request, opts ...grpc.CallOption) (*DeleteMemberInvite_Response, error) {
	out := new(DeleteMemberInvite_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_DeleteMemberInvite_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) MemberInviteActivate(ctx context.Context, in *MemberInviteActivate_Request, opts ...grpc.CallOption) (*MemberInviteActivate_Response, error) {
	out := new(MemberInviteActivate_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_MemberInviteActivate_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) ListServerMembers(ctx context.Context, in *ListServerMembers_Request, opts ...grpc.CallOption) (*ListServerMembers_Response, error) {
	out := new(ListServerMembers_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_ListServerMembers_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) ServerMember(ctx context.Context, in *ServerMember_Request, opts ...grpc.CallOption) (*ServerMember_Response, error) {
	out := new(ServerMember_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_ServerMember_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) AddServerMember(ctx context.Context, in *AddServerMember_Request, opts ...grpc.CallOption) (*AddServerMember_Response, error) {
	out := new(AddServerMember_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_AddServerMember_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) UpdateServerMember(ctx context.Context, in *UpdateServerMember_Request, opts ...grpc.CallOption) (*UpdateServerMember_Response, error) {
	out := new(UpdateServerMember_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_UpdateServerMember_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) DeleteServerMember(ctx context.Context, in *DeleteServerMember_Request, opts ...grpc.CallOption) (*DeleteServerMember_Response, error) {
	out := new(DeleteServerMember_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_DeleteServerMember_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *memberHandlersClient) MembersWithoutServer(ctx context.Context, in *MembersWithoutServer_Request, opts ...grpc.CallOption) (*MembersWithoutServer_Response, error) {
	out := new(MembersWithoutServer_Response)
	err := c.cc.Invoke(ctx, MemberHandlers_MembersWithoutServer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MemberHandlersServer is the server API for MemberHandlers service.
// All implementations must embed UnimplementedMemberHandlersServer
// for forward compatibility
type MemberHandlersServer interface {
	// Project section
	ListProjectMembers(context.Context, *ListProjectMembers_Request) (*ListProjectMembers_Response, error)
	ProjectMember(context.Context, *ProjectMember_Request) (*ProjectMember_Response, error)
	AddProjectMember(context.Context, *AddProjectMember_Request) (*AddProjectMember_Response, error)
	UpdateProjectMember(context.Context, *UpdateProjectMember_Request) (*UpdateProjectMember_Response, error)
	DeleteProjectMember(context.Context, *DeleteProjectMember_Request) (*DeleteProjectMember_Response, error)
	// Used in finding and adding a new member to the project
	UsersWithoutProject(context.Context, *UsersWithoutProject_Request) (*UsersWithoutProject_Response, error)
	// Invite section
	ListMembersInvite(context.Context, *ListMembersInvite_Request) (*ListMembersInvite_Response, error)
	AddMemberInvite(context.Context, *AddMemberInvite_Request) (*AddMemberInvite_Response, error)
	DeleteMemberInvite(context.Context, *DeleteMemberInvite_Request) (*DeleteMemberInvite_Response, error)
	MemberInviteActivate(context.Context, *MemberInviteActivate_Request) (*MemberInviteActivate_Response, error)
	// Server section
	ListServerMembers(context.Context, *ListServerMembers_Request) (*ListServerMembers_Response, error)
	ServerMember(context.Context, *ServerMember_Request) (*ServerMember_Response, error)
	AddServerMember(context.Context, *AddServerMember_Request) (*AddServerMember_Response, error)
	UpdateServerMember(context.Context, *UpdateServerMember_Request) (*UpdateServerMember_Response, error)
	DeleteServerMember(context.Context, *DeleteServerMember_Request) (*DeleteServerMember_Response, error)
	// Used in finding and adding a new member to the server
	MembersWithoutServer(context.Context, *MembersWithoutServer_Request) (*MembersWithoutServer_Response, error)
	mustEmbedUnimplementedMemberHandlersServer()
}

// UnimplementedMemberHandlersServer must be embedded to have forward compatible implementations.
type UnimplementedMemberHandlersServer struct {
}

func (UnimplementedMemberHandlersServer) ListProjectMembers(context.Context, *ListProjectMembers_Request) (*ListProjectMembers_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListProjectMembers not implemented")
}
func (UnimplementedMemberHandlersServer) ProjectMember(context.Context, *ProjectMember_Request) (*ProjectMember_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ProjectMember not implemented")
}
func (UnimplementedMemberHandlersServer) AddProjectMember(context.Context, *AddProjectMember_Request) (*AddProjectMember_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddProjectMember not implemented")
}
func (UnimplementedMemberHandlersServer) UpdateProjectMember(context.Context, *UpdateProjectMember_Request) (*UpdateProjectMember_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateProjectMember not implemented")
}
func (UnimplementedMemberHandlersServer) DeleteProjectMember(context.Context, *DeleteProjectMember_Request) (*DeleteProjectMember_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteProjectMember not implemented")
}
func (UnimplementedMemberHandlersServer) UsersWithoutProject(context.Context, *UsersWithoutProject_Request) (*UsersWithoutProject_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UsersWithoutProject not implemented")
}
func (UnimplementedMemberHandlersServer) ListMembersInvite(context.Context, *ListMembersInvite_Request) (*ListMembersInvite_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListMembersInvite not implemented")
}
func (UnimplementedMemberHandlersServer) AddMemberInvite(context.Context, *AddMemberInvite_Request) (*AddMemberInvite_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddMemberInvite not implemented")
}
func (UnimplementedMemberHandlersServer) DeleteMemberInvite(context.Context, *DeleteMemberInvite_Request) (*DeleteMemberInvite_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteMemberInvite not implemented")
}
func (UnimplementedMemberHandlersServer) MemberInviteActivate(context.Context, *MemberInviteActivate_Request) (*MemberInviteActivate_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MemberInviteActivate not implemented")
}
func (UnimplementedMemberHandlersServer) ListServerMembers(context.Context, *ListServerMembers_Request) (*ListServerMembers_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListServerMembers not implemented")
}
func (UnimplementedMemberHandlersServer) ServerMember(context.Context, *ServerMember_Request) (*ServerMember_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerMember not implemented")
}
func (UnimplementedMemberHandlersServer) AddServerMember(context.Context, *AddServerMember_Request) (*AddServerMember_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddServerMember not implemented")
}
func (UnimplementedMemberHandlersServer) UpdateServerMember(context.Context, *UpdateServerMember_Request) (*UpdateServerMember_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServerMember not implemented")
}
func (UnimplementedMemberHandlersServer) DeleteServerMember(context.Context, *DeleteServerMember_Request) (*DeleteServerMember_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteServerMember not implemented")
}
func (UnimplementedMemberHandlersServer) MembersWithoutServer(context.Context, *MembersWithoutServer_Request) (*MembersWithoutServer_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MembersWithoutServer not implemented")
}
func (UnimplementedMemberHandlersServer) mustEmbedUnimplementedMemberHandlersServer() {}

// UnsafeMemberHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to MemberHandlersServer will
// result in compilation errors.
type UnsafeMemberHandlersServer interface {
	mustEmbedUnimplementedMemberHandlersServer()
}

func RegisterMemberHandlersServer(s grpc.ServiceRegistrar, srv MemberHandlersServer) {
	s.RegisterService(&MemberHandlers_ServiceDesc, srv)
}

func _MemberHandlers_ListProjectMembers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListProjectMembers_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).ListProjectMembers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_ListProjectMembers_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).ListProjectMembers(ctx, req.(*ListProjectMembers_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_ProjectMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectMember_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).ProjectMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_ProjectMember_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).ProjectMember(ctx, req.(*ProjectMember_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_AddProjectMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddProjectMember_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).AddProjectMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_AddProjectMember_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).AddProjectMember(ctx, req.(*AddProjectMember_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_UpdateProjectMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateProjectMember_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).UpdateProjectMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_UpdateProjectMember_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).UpdateProjectMember(ctx, req.(*UpdateProjectMember_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_DeleteProjectMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteProjectMember_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).DeleteProjectMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_DeleteProjectMember_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).DeleteProjectMember(ctx, req.(*DeleteProjectMember_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_UsersWithoutProject_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UsersWithoutProject_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).UsersWithoutProject(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_UsersWithoutProject_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).UsersWithoutProject(ctx, req.(*UsersWithoutProject_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_ListMembersInvite_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListMembersInvite_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).ListMembersInvite(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_ListMembersInvite_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).ListMembersInvite(ctx, req.(*ListMembersInvite_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_AddMemberInvite_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddMemberInvite_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).AddMemberInvite(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_AddMemberInvite_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).AddMemberInvite(ctx, req.(*AddMemberInvite_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_DeleteMemberInvite_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteMemberInvite_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).DeleteMemberInvite(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_DeleteMemberInvite_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).DeleteMemberInvite(ctx, req.(*DeleteMemberInvite_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_MemberInviteActivate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MemberInviteActivate_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).MemberInviteActivate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_MemberInviteActivate_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).MemberInviteActivate(ctx, req.(*MemberInviteActivate_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_ListServerMembers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListServerMembers_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).ListServerMembers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_ListServerMembers_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).ListServerMembers(ctx, req.(*ListServerMembers_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_ServerMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerMember_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).ServerMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_ServerMember_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).ServerMember(ctx, req.(*ServerMember_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_AddServerMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddServerMember_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).AddServerMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_AddServerMember_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).AddServerMember(ctx, req.(*AddServerMember_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_UpdateServerMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateServerMember_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).UpdateServerMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_UpdateServerMember_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).UpdateServerMember(ctx, req.(*UpdateServerMember_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_DeleteServerMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteServerMember_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).DeleteServerMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_DeleteServerMember_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).DeleteServerMember(ctx, req.(*DeleteServerMember_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _MemberHandlers_MembersWithoutServer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MembersWithoutServer_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MemberHandlersServer).MembersWithoutServer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MemberHandlers_MembersWithoutServer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MemberHandlersServer).MembersWithoutServer(ctx, req.(*MembersWithoutServer_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// MemberHandlers_ServiceDesc is the grpc.ServiceDesc for MemberHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var MemberHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "member.MemberHandlers",
	HandlerType: (*MemberHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListProjectMembers",
			Handler:    _MemberHandlers_ListProjectMembers_Handler,
		},
		{
			MethodName: "ProjectMember",
			Handler:    _MemberHandlers_ProjectMember_Handler,
		},
		{
			MethodName: "AddProjectMember",
			Handler:    _MemberHandlers_AddProjectMember_Handler,
		},
		{
			MethodName: "UpdateProjectMember",
			Handler:    _MemberHandlers_UpdateProjectMember_Handler,
		},
		{
			MethodName: "DeleteProjectMember",
			Handler:    _MemberHandlers_DeleteProjectMember_Handler,
		},
		{
			MethodName: "UsersWithoutProject",
			Handler:    _MemberHandlers_UsersWithoutProject_Handler,
		},
		{
			MethodName: "ListMembersInvite",
			Handler:    _MemberHandlers_ListMembersInvite_Handler,
		},
		{
			MethodName: "AddMemberInvite",
			Handler:    _MemberHandlers_AddMemberInvite_Handler,
		},
		{
			MethodName: "DeleteMemberInvite",
			Handler:    _MemberHandlers_DeleteMemberInvite_Handler,
		},
		{
			MethodName: "MemberInviteActivate",
			Handler:    _MemberHandlers_MemberInviteActivate_Handler,
		},
		{
			MethodName: "ListServerMembers",
			Handler:    _MemberHandlers_ListServerMembers_Handler,
		},
		{
			MethodName: "ServerMember",
			Handler:    _MemberHandlers_ServerMember_Handler,
		},
		{
			MethodName: "AddServerMember",
			Handler:    _MemberHandlers_AddServerMember_Handler,
		},
		{
			MethodName: "UpdateServerMember",
			Handler:    _MemberHandlers_UpdateServerMember_Handler,
		},
		{
			MethodName: "DeleteServerMember",
			Handler:    _MemberHandlers_DeleteServerMember_Handler,
		},
		{
			MethodName: "MembersWithoutServer",
			Handler:    _MemberHandlers_MembersWithoutServer_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "member.proto",
}
