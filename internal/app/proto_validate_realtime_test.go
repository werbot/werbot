package app

import (
	"strings"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/assert"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// Stage 13 bounds: ids are uuids, front is ssh or web, a terminate reason
// is 1-256 characters, a close reason at most 32 bytes. Data and resize
// frames carry no rules: an out-of-bounds window is skipped in code and the
// stream lives.
func TestRequestValidationRealtime(t *testing.T) {
	id := "0190a000-0000-7000-8000-000000000001"
	web := func(ticket string) *gatewayv1.AuthenticateUserRequest {
		return &gatewayv1.AuthenticateUserRequest{By: &gatewayv1.AuthenticateUserRequest_WebTicket{WebTicket: ticket}, ClientIp: "0.0.0.0"}
	}
	assert.NoError(t, protovalidate.Validate(web(id)))
	assert.Error(t, protovalidate.Validate(web("x")))

	start := func(front string) *gatewayv1.StartSessionRequest {
		return &gatewayv1.StartSessionRequest{UserId: id, ResourceId: id, ClientIp: "203.0.113.7", Kind: "terminal", Front: front}
	}
	assert.NoError(t, protovalidate.Validate(start("")))
	assert.NoError(t, protovalidate.Validate(start("ssh")))
	assert.NoError(t, protovalidate.Validate(start("web")))
	assert.Error(t, protovalidate.Validate(start("tcp")))

	terminate := func(reason string) *werbotv1.TerminateSessionRequest {
		return &werbotv1.TerminateSessionRequest{Id: id, Reason: reason}
	}
	assert.NoError(t, protovalidate.Validate(terminate(strings.Repeat("r", 256))))
	assert.Error(t, protovalidate.Validate(terminate(strings.Repeat("r", 257))))
	assert.Error(t, protovalidate.Validate(terminate("")))
	assert.Error(t, protovalidate.Validate(&werbotv1.TerminateSessionRequest{Id: "x", Reason: "r"}))

	frame := func(f *gatewayv1.StreamFrame) *gatewayv1.StreamRequest { return &gatewayv1.StreamRequest{Frame: f} }
	open := func(ticket string) *gatewayv1.StreamFrame {
		return &gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Open{Open: &gatewayv1.StreamOpen{TicketId: ticket}}}
	}
	closed := func(reason string) *gatewayv1.StreamFrame {
		return &gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Close{Close: &gatewayv1.StreamClose{Reason: reason}}}
	}
	assert.NoError(t, protovalidate.Validate(frame(open(id))))
	assert.Error(t, protovalidate.Validate(frame(open("x"))))
	assert.NoError(t, protovalidate.Validate(frame(closed(strings.Repeat("c", 32)))))
	assert.Error(t, protovalidate.Validate(&gatewayv1.StreamResponse{Frame: closed(strings.Repeat("c", 33))}))
	assert.NoError(t, protovalidate.Validate(frame(&gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Resize{
		Resize: &gatewayv1.Resize{Cols: 10000, Rows: -1}}})), "a resize out of bounds is skipped in code")
	assert.NoError(t, protovalidate.Validate(frame(&gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Data{
		Data: make([]byte, 64<<10)}})), "data has no rule")
}
