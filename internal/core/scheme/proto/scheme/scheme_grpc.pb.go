// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// source: scheme.proto

package scheme

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
	SchemeHandlers_Schemes_FullMethodName              = "/scheme.SchemeHandlers/Schemes"
	SchemeHandlers_Scheme_FullMethodName               = "/scheme.SchemeHandlers/Scheme"
	SchemeHandlers_AddScheme_FullMethodName            = "/scheme.SchemeHandlers/AddScheme"
	SchemeHandlers_UpdateScheme_FullMethodName         = "/scheme.SchemeHandlers/UpdateScheme"
	SchemeHandlers_DeleteScheme_FullMethodName         = "/scheme.SchemeHandlers/DeleteScheme"
	SchemeHandlers_SchemeAccess_FullMethodName         = "/scheme.SchemeHandlers/SchemeAccess"
	SchemeHandlers_SchemeActivity_FullMethodName       = "/scheme.SchemeHandlers/SchemeActivity"
	SchemeHandlers_UpdateSchemeActivity_FullMethodName = "/scheme.SchemeHandlers/UpdateSchemeActivity"
	SchemeHandlers_SchemeFirewall_FullMethodName       = "/scheme.SchemeHandlers/SchemeFirewall"
	SchemeHandlers_AddSchemeFirewall_FullMethodName    = "/scheme.SchemeHandlers/AddSchemeFirewall"
	SchemeHandlers_UpdateSchemeFirewall_FullMethodName = "/scheme.SchemeHandlers/UpdateSchemeFirewall"
	SchemeHandlers_DeleteSchemeFirewall_FullMethodName = "/scheme.SchemeHandlers/DeleteSchemeFirewall"
	SchemeHandlers_ProfileSchemes_FullMethodName       = "/scheme.SchemeHandlers/ProfileSchemes"
	SchemeHandlers_SystemSchemesByAlias_FullMethodName = "/scheme.SchemeHandlers/SystemSchemesByAlias"
	SchemeHandlers_SystemSchemeAccess_FullMethodName   = "/scheme.SchemeHandlers/SystemSchemeAccess"
	SchemeHandlers_SystemHostKey_FullMethodName        = "/scheme.SchemeHandlers/SystemHostKey"
	SchemeHandlers_SystemUpdateHostKey_FullMethodName  = "/scheme.SchemeHandlers/SystemUpdateHostKey"
)

// SchemeHandlersClient is the client API for SchemeHandlers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SchemeHandlersClient interface {
	// Scheme section
	Schemes(ctx context.Context, in *Schemes_Request, opts ...grpc.CallOption) (*Schemes_Response, error)
	Scheme(ctx context.Context, in *Scheme_Request, opts ...grpc.CallOption) (*Scheme_Response, error)
	AddScheme(ctx context.Context, in *AddScheme_Request, opts ...grpc.CallOption) (*AddScheme_Response, error)
	UpdateScheme(ctx context.Context, in *UpdateScheme_Request, opts ...grpc.CallOption) (*UpdateScheme_Response, error)
	DeleteScheme(ctx context.Context, in *DeleteScheme_Request, opts ...grpc.CallOption) (*DeleteScheme_Response, error)
	// Scheme Access section
	SchemeAccess(ctx context.Context, in *SchemeAccess_Request, opts ...grpc.CallOption) (*SchemeAccess_Response, error)
	// Scheme Activity section
	SchemeActivity(ctx context.Context, in *SchemeActivity_Request, opts ...grpc.CallOption) (*SchemeActivity_Response, error)
	UpdateSchemeActivity(ctx context.Context, in *UpdateSchemeActivity_Request, opts ...grpc.CallOption) (*UpdateSchemeActivity_Response, error)
	// Scheme Firewall section
	SchemeFirewall(ctx context.Context, in *SchemeFirewall_Request, opts ...grpc.CallOption) (*SchemeFirewall_Response, error)
	AddSchemeFirewall(ctx context.Context, in *AddSchemeFirewall_Request, opts ...grpc.CallOption) (*AddSchemeFirewall_Response, error)
	UpdateSchemeFirewall(ctx context.Context, in *UpdateSchemeFirewall_Request, opts ...grpc.CallOption) (*UpdateSchemeFirewall_Response, error)
	DeleteSchemeFirewall(ctx context.Context, in *DeleteSchemeFirewall_Request, opts ...grpc.CallOption) (*DeleteSchemeFirewall_Response, error)
	// All user shared schemes
	ProfileSchemes(ctx context.Context, in *ProfileSchemes_Request, opts ...grpc.CallOption) (*ProfileSchemes_Response, error)
	// SYSTEM methods, using only in workers !!!!
	SystemSchemesByAlias(ctx context.Context, in *SystemSchemesByAlias_Request, opts ...grpc.CallOption) (*SystemSchemesByAlias_Response, error)
	SystemSchemeAccess(ctx context.Context, in *SystemSchemeAccess_Request, opts ...grpc.CallOption) (*SystemSchemeAccess_Response, error)
	SystemHostKey(ctx context.Context, in *SystemHostKey_Request, opts ...grpc.CallOption) (*SystemHostKey_Response, error)
	SystemUpdateHostKey(ctx context.Context, in *SystemUpdateHostKey_Request, opts ...grpc.CallOption) (*SystemUpdateHostKey_Response, error)
}

type schemeHandlersClient struct {
	cc grpc.ClientConnInterface
}

func NewSchemeHandlersClient(cc grpc.ClientConnInterface) SchemeHandlersClient {
	return &schemeHandlersClient{cc}
}

func (c *schemeHandlersClient) Schemes(ctx context.Context, in *Schemes_Request, opts ...grpc.CallOption) (*Schemes_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Schemes_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_Schemes_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) Scheme(ctx context.Context, in *Scheme_Request, opts ...grpc.CallOption) (*Scheme_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Scheme_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_Scheme_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) AddScheme(ctx context.Context, in *AddScheme_Request, opts ...grpc.CallOption) (*AddScheme_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AddScheme_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_AddScheme_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) UpdateScheme(ctx context.Context, in *UpdateScheme_Request, opts ...grpc.CallOption) (*UpdateScheme_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdateScheme_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_UpdateScheme_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) DeleteScheme(ctx context.Context, in *DeleteScheme_Request, opts ...grpc.CallOption) (*DeleteScheme_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteScheme_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_DeleteScheme_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) SchemeAccess(ctx context.Context, in *SchemeAccess_Request, opts ...grpc.CallOption) (*SchemeAccess_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SchemeAccess_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_SchemeAccess_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) SchemeActivity(ctx context.Context, in *SchemeActivity_Request, opts ...grpc.CallOption) (*SchemeActivity_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SchemeActivity_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_SchemeActivity_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) UpdateSchemeActivity(ctx context.Context, in *UpdateSchemeActivity_Request, opts ...grpc.CallOption) (*UpdateSchemeActivity_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdateSchemeActivity_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_UpdateSchemeActivity_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) SchemeFirewall(ctx context.Context, in *SchemeFirewall_Request, opts ...grpc.CallOption) (*SchemeFirewall_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SchemeFirewall_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_SchemeFirewall_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) AddSchemeFirewall(ctx context.Context, in *AddSchemeFirewall_Request, opts ...grpc.CallOption) (*AddSchemeFirewall_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AddSchemeFirewall_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_AddSchemeFirewall_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) UpdateSchemeFirewall(ctx context.Context, in *UpdateSchemeFirewall_Request, opts ...grpc.CallOption) (*UpdateSchemeFirewall_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdateSchemeFirewall_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_UpdateSchemeFirewall_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) DeleteSchemeFirewall(ctx context.Context, in *DeleteSchemeFirewall_Request, opts ...grpc.CallOption) (*DeleteSchemeFirewall_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteSchemeFirewall_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_DeleteSchemeFirewall_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) ProfileSchemes(ctx context.Context, in *ProfileSchemes_Request, opts ...grpc.CallOption) (*ProfileSchemes_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ProfileSchemes_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_ProfileSchemes_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) SystemSchemesByAlias(ctx context.Context, in *SystemSchemesByAlias_Request, opts ...grpc.CallOption) (*SystemSchemesByAlias_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SystemSchemesByAlias_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_SystemSchemesByAlias_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) SystemSchemeAccess(ctx context.Context, in *SystemSchemeAccess_Request, opts ...grpc.CallOption) (*SystemSchemeAccess_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SystemSchemeAccess_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_SystemSchemeAccess_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) SystemHostKey(ctx context.Context, in *SystemHostKey_Request, opts ...grpc.CallOption) (*SystemHostKey_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SystemHostKey_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_SystemHostKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemeHandlersClient) SystemUpdateHostKey(ctx context.Context, in *SystemUpdateHostKey_Request, opts ...grpc.CallOption) (*SystemUpdateHostKey_Response, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SystemUpdateHostKey_Response)
	err := c.cc.Invoke(ctx, SchemeHandlers_SystemUpdateHostKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SchemeHandlersServer is the server API for SchemeHandlers service.
// All implementations must embed UnimplementedSchemeHandlersServer
// for forward compatibility.
type SchemeHandlersServer interface {
	// Scheme section
	Schemes(context.Context, *Schemes_Request) (*Schemes_Response, error)
	Scheme(context.Context, *Scheme_Request) (*Scheme_Response, error)
	AddScheme(context.Context, *AddScheme_Request) (*AddScheme_Response, error)
	UpdateScheme(context.Context, *UpdateScheme_Request) (*UpdateScheme_Response, error)
	DeleteScheme(context.Context, *DeleteScheme_Request) (*DeleteScheme_Response, error)
	// Scheme Access section
	SchemeAccess(context.Context, *SchemeAccess_Request) (*SchemeAccess_Response, error)
	// Scheme Activity section
	SchemeActivity(context.Context, *SchemeActivity_Request) (*SchemeActivity_Response, error)
	UpdateSchemeActivity(context.Context, *UpdateSchemeActivity_Request) (*UpdateSchemeActivity_Response, error)
	// Scheme Firewall section
	SchemeFirewall(context.Context, *SchemeFirewall_Request) (*SchemeFirewall_Response, error)
	AddSchemeFirewall(context.Context, *AddSchemeFirewall_Request) (*AddSchemeFirewall_Response, error)
	UpdateSchemeFirewall(context.Context, *UpdateSchemeFirewall_Request) (*UpdateSchemeFirewall_Response, error)
	DeleteSchemeFirewall(context.Context, *DeleteSchemeFirewall_Request) (*DeleteSchemeFirewall_Response, error)
	// All user shared schemes
	ProfileSchemes(context.Context, *ProfileSchemes_Request) (*ProfileSchemes_Response, error)
	// SYSTEM methods, using only in workers !!!!
	SystemSchemesByAlias(context.Context, *SystemSchemesByAlias_Request) (*SystemSchemesByAlias_Response, error)
	SystemSchemeAccess(context.Context, *SystemSchemeAccess_Request) (*SystemSchemeAccess_Response, error)
	SystemHostKey(context.Context, *SystemHostKey_Request) (*SystemHostKey_Response, error)
	SystemUpdateHostKey(context.Context, *SystemUpdateHostKey_Request) (*SystemUpdateHostKey_Response, error)
	mustEmbedUnimplementedSchemeHandlersServer()
}

// UnimplementedSchemeHandlersServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedSchemeHandlersServer struct{}

func (UnimplementedSchemeHandlersServer) Schemes(context.Context, *Schemes_Request) (*Schemes_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Schemes not implemented")
}
func (UnimplementedSchemeHandlersServer) Scheme(context.Context, *Scheme_Request) (*Scheme_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Scheme not implemented")
}
func (UnimplementedSchemeHandlersServer) AddScheme(context.Context, *AddScheme_Request) (*AddScheme_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddScheme not implemented")
}
func (UnimplementedSchemeHandlersServer) UpdateScheme(context.Context, *UpdateScheme_Request) (*UpdateScheme_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateScheme not implemented")
}
func (UnimplementedSchemeHandlersServer) DeleteScheme(context.Context, *DeleteScheme_Request) (*DeleteScheme_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteScheme not implemented")
}
func (UnimplementedSchemeHandlersServer) SchemeAccess(context.Context, *SchemeAccess_Request) (*SchemeAccess_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SchemeAccess not implemented")
}
func (UnimplementedSchemeHandlersServer) SchemeActivity(context.Context, *SchemeActivity_Request) (*SchemeActivity_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SchemeActivity not implemented")
}
func (UnimplementedSchemeHandlersServer) UpdateSchemeActivity(context.Context, *UpdateSchemeActivity_Request) (*UpdateSchemeActivity_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateSchemeActivity not implemented")
}
func (UnimplementedSchemeHandlersServer) SchemeFirewall(context.Context, *SchemeFirewall_Request) (*SchemeFirewall_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SchemeFirewall not implemented")
}
func (UnimplementedSchemeHandlersServer) AddSchemeFirewall(context.Context, *AddSchemeFirewall_Request) (*AddSchemeFirewall_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddSchemeFirewall not implemented")
}
func (UnimplementedSchemeHandlersServer) UpdateSchemeFirewall(context.Context, *UpdateSchemeFirewall_Request) (*UpdateSchemeFirewall_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateSchemeFirewall not implemented")
}
func (UnimplementedSchemeHandlersServer) DeleteSchemeFirewall(context.Context, *DeleteSchemeFirewall_Request) (*DeleteSchemeFirewall_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteSchemeFirewall not implemented")
}
func (UnimplementedSchemeHandlersServer) ProfileSchemes(context.Context, *ProfileSchemes_Request) (*ProfileSchemes_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ProfileSchemes not implemented")
}
func (UnimplementedSchemeHandlersServer) SystemSchemesByAlias(context.Context, *SystemSchemesByAlias_Request) (*SystemSchemesByAlias_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SystemSchemesByAlias not implemented")
}
func (UnimplementedSchemeHandlersServer) SystemSchemeAccess(context.Context, *SystemSchemeAccess_Request) (*SystemSchemeAccess_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SystemSchemeAccess not implemented")
}
func (UnimplementedSchemeHandlersServer) SystemHostKey(context.Context, *SystemHostKey_Request) (*SystemHostKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SystemHostKey not implemented")
}
func (UnimplementedSchemeHandlersServer) SystemUpdateHostKey(context.Context, *SystemUpdateHostKey_Request) (*SystemUpdateHostKey_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SystemUpdateHostKey not implemented")
}
func (UnimplementedSchemeHandlersServer) mustEmbedUnimplementedSchemeHandlersServer() {}
func (UnimplementedSchemeHandlersServer) testEmbeddedByValue()                        {}

// UnsafeSchemeHandlersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SchemeHandlersServer will
// result in compilation errors.
type UnsafeSchemeHandlersServer interface {
	mustEmbedUnimplementedSchemeHandlersServer()
}

func RegisterSchemeHandlersServer(s grpc.ServiceRegistrar, srv SchemeHandlersServer) {
	// If the following call pancis, it indicates UnimplementedSchemeHandlersServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&SchemeHandlers_ServiceDesc, srv)
}

func _SchemeHandlers_Schemes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Schemes_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).Schemes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_Schemes_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).Schemes(ctx, req.(*Schemes_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_Scheme_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Scheme_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).Scheme(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_Scheme_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).Scheme(ctx, req.(*Scheme_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_AddScheme_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddScheme_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).AddScheme(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_AddScheme_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).AddScheme(ctx, req.(*AddScheme_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_UpdateScheme_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateScheme_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).UpdateScheme(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_UpdateScheme_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).UpdateScheme(ctx, req.(*UpdateScheme_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_DeleteScheme_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteScheme_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).DeleteScheme(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_DeleteScheme_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).DeleteScheme(ctx, req.(*DeleteScheme_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_SchemeAccess_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SchemeAccess_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).SchemeAccess(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_SchemeAccess_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).SchemeAccess(ctx, req.(*SchemeAccess_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_SchemeActivity_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SchemeActivity_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).SchemeActivity(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_SchemeActivity_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).SchemeActivity(ctx, req.(*SchemeActivity_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_UpdateSchemeActivity_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateSchemeActivity_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).UpdateSchemeActivity(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_UpdateSchemeActivity_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).UpdateSchemeActivity(ctx, req.(*UpdateSchemeActivity_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_SchemeFirewall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SchemeFirewall_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).SchemeFirewall(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_SchemeFirewall_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).SchemeFirewall(ctx, req.(*SchemeFirewall_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_AddSchemeFirewall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddSchemeFirewall_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).AddSchemeFirewall(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_AddSchemeFirewall_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).AddSchemeFirewall(ctx, req.(*AddSchemeFirewall_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_UpdateSchemeFirewall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateSchemeFirewall_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).UpdateSchemeFirewall(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_UpdateSchemeFirewall_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).UpdateSchemeFirewall(ctx, req.(*UpdateSchemeFirewall_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_DeleteSchemeFirewall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteSchemeFirewall_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).DeleteSchemeFirewall(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_DeleteSchemeFirewall_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).DeleteSchemeFirewall(ctx, req.(*DeleteSchemeFirewall_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_ProfileSchemes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProfileSchemes_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).ProfileSchemes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_ProfileSchemes_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).ProfileSchemes(ctx, req.(*ProfileSchemes_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_SystemSchemesByAlias_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SystemSchemesByAlias_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).SystemSchemesByAlias(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_SystemSchemesByAlias_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).SystemSchemesByAlias(ctx, req.(*SystemSchemesByAlias_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_SystemSchemeAccess_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SystemSchemeAccess_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).SystemSchemeAccess(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_SystemSchemeAccess_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).SystemSchemeAccess(ctx, req.(*SystemSchemeAccess_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_SystemHostKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SystemHostKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).SystemHostKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_SystemHostKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).SystemHostKey(ctx, req.(*SystemHostKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemeHandlers_SystemUpdateHostKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SystemUpdateHostKey_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemeHandlersServer).SystemUpdateHostKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemeHandlers_SystemUpdateHostKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemeHandlersServer).SystemUpdateHostKey(ctx, req.(*SystemUpdateHostKey_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// SchemeHandlers_ServiceDesc is the grpc.ServiceDesc for SchemeHandlers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SchemeHandlers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "scheme.SchemeHandlers",
	HandlerType: (*SchemeHandlersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Schemes",
			Handler:    _SchemeHandlers_Schemes_Handler,
		},
		{
			MethodName: "Scheme",
			Handler:    _SchemeHandlers_Scheme_Handler,
		},
		{
			MethodName: "AddScheme",
			Handler:    _SchemeHandlers_AddScheme_Handler,
		},
		{
			MethodName: "UpdateScheme",
			Handler:    _SchemeHandlers_UpdateScheme_Handler,
		},
		{
			MethodName: "DeleteScheme",
			Handler:    _SchemeHandlers_DeleteScheme_Handler,
		},
		{
			MethodName: "SchemeAccess",
			Handler:    _SchemeHandlers_SchemeAccess_Handler,
		},
		{
			MethodName: "SchemeActivity",
			Handler:    _SchemeHandlers_SchemeActivity_Handler,
		},
		{
			MethodName: "UpdateSchemeActivity",
			Handler:    _SchemeHandlers_UpdateSchemeActivity_Handler,
		},
		{
			MethodName: "SchemeFirewall",
			Handler:    _SchemeHandlers_SchemeFirewall_Handler,
		},
		{
			MethodName: "AddSchemeFirewall",
			Handler:    _SchemeHandlers_AddSchemeFirewall_Handler,
		},
		{
			MethodName: "UpdateSchemeFirewall",
			Handler:    _SchemeHandlers_UpdateSchemeFirewall_Handler,
		},
		{
			MethodName: "DeleteSchemeFirewall",
			Handler:    _SchemeHandlers_DeleteSchemeFirewall_Handler,
		},
		{
			MethodName: "ProfileSchemes",
			Handler:    _SchemeHandlers_ProfileSchemes_Handler,
		},
		{
			MethodName: "SystemSchemesByAlias",
			Handler:    _SchemeHandlers_SystemSchemesByAlias_Handler,
		},
		{
			MethodName: "SystemSchemeAccess",
			Handler:    _SchemeHandlers_SystemSchemeAccess_Handler,
		},
		{
			MethodName: "SystemHostKey",
			Handler:    _SchemeHandlers_SystemHostKey_Handler,
		},
		{
			MethodName: "SystemUpdateHostKey",
			Handler:    _SchemeHandlers_SystemUpdateHostKey_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "scheme.proto",
}
