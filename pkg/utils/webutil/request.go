package webutil

import (
	"github.com/gofiber/fiber/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Request wraps the context and protobuf message.
type Request struct {
	ctx *fiber.Ctx
	req protoreflect.ProtoMessage
}

// Parse initializes a new Request instance.
func Parse(c *fiber.Ctx, request protoreflect.ProtoMessage) *Request {
	return &Request{
		ctx: c,
		req: request,
	}
}

// Body parses the body of the request into the protobuf message.
// If proto is true, it uses protojson.Unmarshal; otherwise, it uses BodyParser.
func (r *Request) Body(useProto ...bool) error {
	if err := r.ctx.BodyParser(r.req); err != nil {
		return err
	}

	if len(useProto) > 0 && useProto[0] {
		tmpReq := proto.Clone(r.req)
		if err := protojson.Unmarshal(r.ctx.Body(), tmpReq); err != nil {
			return err
		}
		proto.Merge(tmpReq, r.req)
		proto.Merge(r.req, tmpReq) // this code for record old value, r.req = tmpReq - don't work :(
	}

	return nil
}

// Query parses the query parameters of the request.
func (r *Request) Query() error {
	return r.ctx.QueryParser(r.req)
}
