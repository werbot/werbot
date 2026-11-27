package app

import (
	"strings"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// TestRequestValidationTunnelReadonly covers the bounds of the tunnel's
// stage 18 fields (Ruling G10): a check result is ok, has_write, error or
// none, its detail at most 512 bytes, its version not negative; the write
// credential names a session, an approval and a SHA-256; the probe slot is
// primary or agent_readonly (the handler serves only agent_readonly).
func TestRequestValidationTunnelReadonly(t *testing.T) {
	id := "0190a000-0000-7000-8000-000000000001"
	valid := func(m proto.Message, name string) { assert.NoError(t, protovalidate.Validate(m), name) }
	invalid := func(m proto.Message, name string) { assert.Error(t, protovalidate.Validate(m), name) }

	health := func(status, detail string, version int64) *gatewayv1.ResourceHealth {
		return &gatewayv1.ResourceHealth{ResourceId: id, AgentReadonlyStatus: status, AgentReadonlyDetail: detail, AgentReadonlyVersion: version}
	}
	for _, s := range []string{"", "ok", "has_write", "error"} {
		valid(health(s, "", 1), "status "+s)
	}
	for _, s := range []string{"none", "pending", "OK"} {
		invalid(health(s, "", 1), "status "+s)
	}
	valid(health("has_write", strings.Repeat("d", 512), 1), "512 bytes of detail")
	invalid(health("has_write", strings.Repeat("d", 513), 1), "513 bytes of detail")
	invalid(health("ok", "", -1), "a negative version")

	write := func(session, approval string, sum []byte) *gatewayv1.GetWriteCredentialRequest {
		return &gatewayv1.GetWriteCredentialRequest{SessionId: session, ApprovalId: approval, OpSha256: sum}
	}
	valid(write(id, id, make([]byte, 32)), "a write")
	invalid(write("x", id, make([]byte, 32)), "session_id not a UUID")
	invalid(write(id, "", make([]byte, 32)), "no approval_id")
	invalid(write(id, id, nil), "no hash")
	invalid(write(id, id, make([]byte, 33)), "33 bytes")

	valid(&gatewayv1.GetProbeCredentialRequest{ResourceId: id, Slot: "agent_readonly"}, "the agent slot")
	invalid(&gatewayv1.GetProbeCredentialRequest{ResourceId: id, Slot: "x"}, "an unknown slot")
	invalid(&gatewayv1.GetProbeCredentialRequest{ResourceId: "x", Slot: "agent_readonly"}, "resource_id not a UUID")
}
