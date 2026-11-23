package app

import (
	"strings"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// TestRequestValidationTunnelAgents covers the bounds of the tunnel's agent
// fields (stage 12): ids are UUIDs or empty, the output hash is 32 bytes or
// empty, RecordDenial is bounded like a heartbeat denial.
func TestRequestValidationTunnelAgents(t *testing.T) {
	id := "0190a000-0000-7000-8000-000000000001"
	valid := func(m proto.Message, name string) { assert.NoError(t, protovalidate.Validate(m), name) }
	invalid := func(m proto.Message, name string) { assert.Error(t, protovalidate.Validate(m), name) }

	cert := func(agent, run string) *gatewayv1.CertIdentity {
		return &gatewayv1.CertIdentity{UserId: id, CliSessionId: id, Login: "alice", AgentId: agent, RunId: run}
	}
	valid(cert("", ""), "a person's certificate")
	valid(cert(id, id), "an agent certificate")
	invalid(cert("x", id), "agent_id not a UUID")
	invalid(cert(id, "x"), "run_id not a UUID")

	start := func(run, call string) *gatewayv1.StartSessionRequest {
		return &gatewayv1.StartSessionRequest{UserId: id, ResourceId: id, ClientIp: "10.0.0.1", Kind: "exec", AgentRunId: run, ToolCallId: call}
	}
	valid(start("", ""), "a person's session")
	valid(start(id, id), "an agent's session")
	invalid(start("x", ""), "agent_run_id not a UUID")
	invalid(start(id, "x"), "tool_call_id not a UUID")

	end := func(sum []byte) *gatewayv1.EndSessionRequest {
		zero := int32(0)
		return &gatewayv1.EndSessionRequest{SessionId: id, Reason: "closed", ExitCode: &zero, OutputSha256: sum}
	}
	valid(end(nil), "no output hash")
	valid(end(make([]byte, 32)), "a SHA-256")
	invalid(end(make([]byte, 31)), "31 bytes")
	invalid(end(make([]byte, 33)), "33 bytes")

	valid(&gatewayv1.Denial{Reason: "denied", AgentRunId: id}, "a heartbeat denial of a run")
	invalid(&gatewayv1.Denial{Reason: "denied", AgentRunId: "x"}, "agent_run_id not a UUID")

	classes := []string{"read", "write", "destructive", "privilege", "secrets", "egress", "opaque"}
	deny := func(mut func(*gatewayv1.RecordDenialRequest)) *gatewayv1.RecordDenialRequest {
		r := &gatewayv1.RecordDenialRequest{RunId: id, Resource: strings.Repeat("r", 63), Command: strings.Repeat("c", 1024),
			Classes: classes, Reasons: make([]string, 16)}
		for i := range r.Reasons {
			r.Reasons[i] = strings.Repeat("x", 128)
		}
		mut(r)
		return r
	}
	valid(deny(func(*gatewayv1.RecordDenialRequest) {}), "every field at its bound")
	for name, mut := range map[string]func(*gatewayv1.RecordDenialRequest){
		"empty run_id":  func(r *gatewayv1.RecordDenialRequest) { r.RunId = "" },
		"bad run_id":    func(r *gatewayv1.RecordDenialRequest) { r.RunId = "x" },
		"long resource": func(r *gatewayv1.RecordDenialRequest) { r.Resource += "r" },
		"long command":  func(r *gatewayv1.RecordDenialRequest) { r.Command += "c" },
		"eight classes": func(r *gatewayv1.RecordDenialRequest) { r.Classes = append(r.Classes, "read") },
		"unknown class": func(r *gatewayv1.RecordDenialRequest) { r.Classes = []string{"shell"} },
		"17 reasons":    func(r *gatewayv1.RecordDenialRequest) { r.Reasons = append(r.Reasons, "x") },
		"long reason":   func(r *gatewayv1.RecordDenialRequest) { r.Reasons[0] += "x" },
	} {
		invalid(deny(mut), name)
	}
}
