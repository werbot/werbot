// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// source: project.proto

package project

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
	ProjectHandlers_Projects_FullMethodName         = "/project.ProjectHandlers/Projects"
	ProjectHandlers_Project_FullMethodName          = "/project.ProjectHandlers/Project"
	ProjectHandlers_AddProject_FullMethodName       = "/project.ProjectHandlers/AddProject"
	ProjectHandlers_UpdateProject_FullMethodName    = "/project.ProjectHandlers/UpdateProject"
	ProjectHandlers_DeleteProject_FullMethodName    = "/project.ProjectHandlers/DeleteProject"
	ProjectHandlers_ProjectKeys_FullMethodName      = "/project.ProjectHandlers/ProjectKeys"
	ProjectHandlers_ProjectKey_FullMethodName       = "/project.ProjectHandlers/ProjectKey"
	ProjectHandlers_AddProjectKey_FullMethodName    = "/project.ProjectHandlers/AddProjectKey"
	ProjectHandlers_DeleteProjectKey_FullMethodName = "/project.ProjectHandlers/DeleteProjectKey"
)

// ProjectHandlersClient is the client API for ProjectHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ProjectHandlersClient interface {
	// Project section
	Projects(ctx context.Context, in *Projects_Request, opts ...grpc.CallOption) (*Projects_Response, error)
	Project(ctx context.Context, in *Project_Request, opts ...grpc.CallOption) (*Project_Response, error)
	AddProject(ctx context.Context, in *AddProject_Request, opts ...grpc.CallOption) (*AddProject_Response, error)
	UpdateProject(ctx context.Context, in *UpdateProject_Request, opts ...grpc.CallOption) (*UpdateProject_Response, error)
	DeleteProject(ctx context.Context, in *DeleteProject_Request, opts ...grpc.CallOption) (*DeleteProject_Response, error)
	// Project key section
	ProjectKeys(ctx context.Context, in *ProjectKeys_Request, opts ...grpc.CallOption) (*ProjectKeys_Response, error)
	ProjectKey(ctx context.Context, in *ProjectKey_Request, opts ...grpc.CallOption) (*ProjectKey_Response, error)
	AddProjectKey(ctx context.Context, in *AddProjectKey_Request, opts ...grpc.CallOption) (*AddProjectKey_Response, error)
	DeleteProjectKey(ctx context.Context, in *DeleteProjectKey_Request, opts ...grpc.CallOption) (*DeleteProjectKey_Response, error)
}

type projectHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewProjectHandlersClient(cc grpc.ClientConnInterface) ProjectHandlersClient {
	return &projectHandlersClient{cc}
}

func (c *projectHandlersClient) Projects(ctx context.Context, in *Projects_Request, opts ...grpc.CallOption) (*Projects_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Projects_Response)
	err := c.cc.Invoke(ctx, ProjectHandlers_Projects_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *projectHandlersClient) Project(ctx context.Context, in *Project_Request, opts ...grpc.CallOption) (*Project_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Project_Response)
	err := c.cc.Invoke(ctx, ProjectHandlers_Project_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *projectHandlersClient) AddProject(ctx context.Context, in *AddProject_Request, opts ...grpc.CallOption) (*AddProject_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AddProject_Response)
	err := c.cc.Invoke(ctx, ProjectHandlers_AddProject_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *projectHandlersClient) UpdateProject(ctx context.Context, in *UpdateProject_Request, opts ...grpc.CallOption) (*UpdateProject_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdateProject_Response)
	err := c.cc.Invoke(ctx, ProjectHandlers_UpdateProject_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *projectHandlersClient) DeleteProject(ctx context.Context, in *DeleteProject_Request, opts ...grpc.CallOption) (*DeleteProject_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteProject_Response)
	err := c.cc.Invoke(ctx, ProjectHandlers_DeleteProject_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *projectHandlersClient) ProjectKeys(ctx context.Context, in *ProjectKeys_Request, opts ...grpc.CallOption) (*ProjectKeys_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ProjectKeys_Response)
	err := c.cc.Invoke(ctx, ProjectHandlers_ProjectKeys_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *projectHandlersClient) ProjectKey(ctx context.Context, in *ProjectKey_Request, opts ...grpc.CallOption) (*ProjectKey_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ProjectKey_Response)
	err := c.cc.Invoke(ctx, ProjectHandlers_ProjectKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *projectHandlersClient) AddProjectKey(ctx context.Context, in *AddProjectKey_Request, opts ...grpc.CallOption) (*AddProjectKey_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AddProjectKey_Response)
	err := c.cc.Invoke(ctx, ProjectHandlers_AddProjectKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *projectHandlersClient) DeleteProjectKey(ctx context.Context, in *DeleteProjectKey_Request, opts ...grpc.CallOption) (*DeleteProjectKey_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteProjectKey_Response)
	err := c.cc.Invoke(ctx, ProjectHandlers_DeleteProjectKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ProjectHandlersServer is the server API for ProjectHandlers service.
// All implementations must embed UnimplementedProjectHandlersServer
// for forward compatibility.
type ProjectHandlersServer interface {
	// Project section
	Projects(context.Context, *Projects_Request) (*Projects_Response, error)
	Project(context.Context, *Project_Request) (*Project_Response, error)
	AddProject(context.Context, *AddProject_Request) (*AddProject_Response, error)
	UpdateProject(context.Context, *UpdateProject_Request) (*UpdateProject_Response, error)
	DeleteProject(context.Context, *DeleteProject_Request) (*DeleteProject_Response, error)
	// Project key section
	ProjectKeys(context.Context, *ProjectKeys_Request) (*ProjectKeys_Response, error)
	ProjectKey(context.Context, *ProjectKey_Request) (*ProjectKey_Response, error)
	AddProjectKey(context.Context, *AddProjectKey_Request) (*AddProjectKey_Response, error)
	DeleteProjectKey(context.Context, *DeleteProjectKey_Request) (*DeleteProjectKey_Response, error)
	mustEmbedUnimplementedProjectHandlersServer()
}

// UnimplementedProjectHandlersServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedProjectHandlersServer struct{}

func (UnimplementedProjectHandlersServer) Projects(context.Context, *Projects_Request) (*Projects_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Projects not implemented")
}
func (UnimplementedProjectHandlersServer) Project(context.Context, *Project_Request) (*Project_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Project not implemented")
}
func (UnimplementedProjectHandlersServer) AddProject(context.Context, *AddProject_Request) (*AddProject_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddProject not implemented")
}
func (UnimplementedProjectHandlersServer) UpdateProject(context.Context, *UpdateProject_Request) (*UpdateProject_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateProject not implemented")
}
func (UnimplementedProjectHandlersServer) DeleteProject(context.Context, *DeleteProject_Request) (*DeleteProject_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteProject not implemented")
}
func (UnimplementedProjectHandlersServer) ProjectKeys(context.Context, *ProjectKeys_Request) (*ProjectKeys_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ProjectKeys not implemented")
}
func (UnimplementedProjectHandlersServer) ProjectKey(context.Context, *ProjectKey_Request) (*ProjectKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ProjectKey not implemented")
}
func (UnimplementedProjectHandlersServer) AddProjectKey(context.Context, *AddProjectKey_Request) (*AddProjectKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddProjectKey not implemented")
}
func (UnimplementedProjectHandlersServer) DeleteProjectKey(context.Context, *DeleteProjectKey_Request) (*DeleteProjectKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteProjectKey not implemented")
}
func (UnimplementedProjectHandlersServer) mustEmbedUnimplementedProjectHandlersServer() {}
func (UnimplementedProjectHandlersServer) testEmbeddedByValue()                         {}

// UnsafeProjectHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ProjectHandlersServer will
// result in compilation errors.
type UnsafeProjectHandlersServer interface {
	mustEmbedUnimplementedProjectHandlersServer()
}

func RegisterProjectHandlersServer(s grpc.ServiceRegistrar, srv ProjectHandlersServer) {
	// If the following call pancis, it indicates UnimplementedProjectHandlersServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&ProjectHandlers_ServiceDesc, srv)
}

func _ProjectHandlers_Projects_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Projects_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectHandlersServer).Projects(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProjectHandlers_Projects_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectHandlersServer).Projects(ctx, req.(*Projects_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProjectHandlers_Project_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Project_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectHandlersServer).Project(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProjectHandlers_Project_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectHandlersServer).Project(ctx, req.(*Project_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProjectHandlers_AddProject_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddProject_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectHandlersServer).AddProject(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProjectHandlers_AddProject_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectHandlersServer).AddProject(ctx, req.(*AddProject_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProjectHandlers_UpdateProject_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateProject_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectHandlersServer).UpdateProject(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProjectHandlers_UpdateProject_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectHandlersServer).UpdateProject(ctx, req.(*UpdateProject_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProjectHandlers_DeleteProject_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteProject_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectHandlersServer).DeleteProject(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProjectHandlers_DeleteProject_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectHandlersServer).DeleteProject(ctx, req.(*DeleteProject_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProjectHandlers_ProjectKeys_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectKeys_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectHandlersServer).ProjectKeys(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProjectHandlers_ProjectKeys_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectHandlersServer).ProjectKeys(ctx, req.(*ProjectKeys_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProjectHandlers_ProjectKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectHandlersServer).ProjectKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProjectHandlers_ProjectKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectHandlersServer).ProjectKey(ctx, req.(*ProjectKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProjectHandlers_AddProjectKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddProjectKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectHandlersServer).AddProjectKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProjectHandlers_AddProjectKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectHandlersServer).AddProjectKey(ctx, req.(*AddProjectKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProjectHandlers_DeleteProjectKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteProjectKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectHandlersServer).DeleteProjectKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProjectHandlers_DeleteProjectKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectHandlersServer).DeleteProjectKey(ctx, req.(*DeleteProjectKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// ProjectHandlers_ServiceDesc is the grpc.ServiceDesc for ProjectHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ProjectHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "project.ProjectHandlers",
	HandlerType: (*ProjectHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Projects",
			Handler:    _ProjectHandlers_Projects_Handler,
		},
		{
			MethodName: "Project",
			Handler:    _ProjectHandlers_Project_Handler,
		},
		{
			MethodName: "AddProject",
			Handler:    _ProjectHandlers_AddProject_Handler,
		},
		{
			MethodName: "UpdateProject",
			Handler:    _ProjectHandlers_UpdateProject_Handler,
		},
		{
			MethodName: "DeleteProject",
			Handler:    _ProjectHandlers_DeleteProject_Handler,
		},
		{
			MethodName: "ProjectKeys",
			Handler:    _ProjectHandlers_ProjectKeys_Handler,
		},
		{
			MethodName: "ProjectKey",
			Handler:    _ProjectHandlers_ProjectKey_Handler,
		},
		{
			MethodName: "AddProjectKey",
			Handler:    _ProjectHandlers_AddProjectKey_Handler,
		},
		{
			MethodName: "DeleteProjectKey",
			Handler:    _ProjectHandlers_DeleteProjectKey_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "project.proto",
}