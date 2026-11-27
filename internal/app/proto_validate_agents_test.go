package app

import (
	"strings"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
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

// TestRequestValidationAgentService covers the bounds of agent.proto (stage
// 12, database limits stage 18): names, limits, statuses, paging and tool calls.
func TestRequestValidationAgentService(t *testing.T) {
	id := "0190a000-0000-7000-8000-000000000001"
	valid := func(m proto.Message, name string) { assert.NoError(t, protovalidate.Validate(m), name) }
	invalid := func(m proto.Message, name string) { assert.Error(t, protovalidate.Validate(m), name) }

	limits := func(actions, concurrent int32, d time.Duration) *werbotv1.CreateAgentRequest {
		return &werbotv1.CreateAgentRequest{Name: "claude", Limits: &werbotv1.AgentLimits{MaxActions: actions,
			MaxConcurrent: concurrent, MaxDuration: durationpb.New(d)}}
	}
	valid(limits(0, 0, 0), "all defaults")
	valid(limits(10000, 32, time.Hour), "every limit at its maximum")
	valid(limits(1, 1, time.Minute), "every limit at its minimum")
	invalid(limits(10001, 0, 0), "max_actions above 10000")
	invalid(limits(0, 33, 0), "max_concurrent above 32")
	invalid(limits(-1, 0, 0), "negative max_actions")
	invalid(limits(0, 0, 59*time.Second), "max_duration below 1m")
	invalid(limits(0, 0, time.Hour+time.Second), "max_duration above 1h")
	db := func(rows int32, size int64, timeout time.Duration, queries int32) *werbotv1.CreateAgentRequest {
		return &werbotv1.CreateAgentRequest{Name: "claude", Limits: &werbotv1.AgentLimits{MaxRows: rows,
			MaxResultBytes: size, StatementTimeout: durationpb.New(timeout), MaxQueries: queries}}
	}
	valid(db(0, 0, 0, 0), "database limits default")
	valid(db(1, 1024, time.Second, 1), "database limits at their minimum")
	valid(db(100000, 64<<20, 10*time.Minute, 100000), "database limits at their maximum")
	invalid(db(100001, 0, 0, 0), "max_rows above 100000")
	invalid(db(0, 1023, 0, 0), "max_result_bytes below 1 KiB")
	invalid(db(0, 64<<20+1, 0, 0), "max_result_bytes above 64 MiB")
	invalid(db(0, 0, 999*time.Millisecond, 0), "statement_timeout below 1s")
	invalid(db(0, 0, 10*time.Minute+time.Second, 0), "statement_timeout above 10m")
	invalid(db(0, 0, 0, 100001), "max_queries above 100000")
	valid(&werbotv1.CreateAgentRequest{Name: strings.Repeat("a", 63), SponsorUserId: id}, "a 63-character name of a sponsor")
	invalid(&werbotv1.CreateAgentRequest{Name: strings.Repeat("a", 64)}, "a 64-character name")
	invalid(&werbotv1.CreateAgentRequest{Name: "Claude"}, "an upper-case name")
	invalid(&werbotv1.CreateAgentRequest{Name: "claude", SponsorUserId: "x"}, "sponsor_user_id not a UUID")

	valid(&werbotv1.UpdateAgentRequest{Id: id}, "nothing changes")
	valid(&werbotv1.UpdateAgentRequest{Id: id, Status: "frozen"}, "freeze")
	invalid(&werbotv1.UpdateAgentRequest{Id: id, Status: "paused"}, "status paused")
	invalid(&werbotv1.UpdateAgentRequest{Id: "x"}, "id not a UUID")

	valid(&werbotv1.ListRunsRequest{}, "page and size default")
	valid(&werbotv1.ListRunsRequest{AgentId: id, Status: "ended", Page: 2, PageSize: 100}, "every filter")
	invalid(&werbotv1.ListRunsRequest{PageSize: 101}, "page_size above 100")
	invalid(&werbotv1.ListRunsRequest{Status: "frozen"}, "status frozen")

	valid(&werbotv1.RecordToolCallRequest{Tool: "ssh_exec", Traceparent: strings.Repeat("0", 55)}, "a traceparent of 55 bytes")
	invalid(&werbotv1.RecordToolCallRequest{Tool: "ssh_exec", Traceparent: strings.Repeat("0", 56)}, "a traceparent of 56 bytes")
	invalid(&werbotv1.RecordToolCallRequest{Tool: "SSH"}, "an upper-case tool")
	valid(&werbotv1.FinishToolCallRequest{Id: id, Status: "pending_approval"}, "pending_approval")
	invalid(&werbotv1.FinishToolCallRequest{Id: id, Status: "running"}, "status running")
}

// TestRequestValidationAgentProfileService covers the bounds of
// agent_profile.proto (stage 12): ids are UUIDs, ttl is 0 or 1h to 90d, the
// label at most 255 bytes.
func TestRequestValidationAgentProfileService(t *testing.T) {
	id := "0190a000-0000-7000-8000-000000000001"
	valid := func(m proto.Message, name string) { assert.NoError(t, protovalidate.Validate(m), name) }
	invalid := func(m proto.Message, name string) { assert.Error(t, protovalidate.Validate(m), name) }

	create := func(ttl time.Duration, label string) *werbotv1.CreateAgentProfileRequest {
		return &werbotv1.CreateAgentProfileRequest{AgentId: id, Ttl: durationpb.New(ttl), Label: label}
	}
	valid(&werbotv1.CreateAgentProfileRequest{AgentId: id}, "no ttl, no label")
	valid(create(0, strings.Repeat("a", 255)), "ttl 0, a label of 255 bytes")
	valid(create(time.Hour, ""), "ttl 1h")
	valid(create(90*24*time.Hour, ""), "ttl 90d")
	invalid(create(30*time.Minute, ""), "ttl below 1h")
	invalid(create(90*24*time.Hour+time.Second, ""), "ttl above 90d")
	invalid(create(-time.Hour, ""), "negative ttl")
	invalid(create(0, strings.Repeat("a", 256)), "a label of 256 bytes")
	invalid(&werbotv1.CreateAgentProfileRequest{AgentId: "x"}, "agent_id not a UUID")
	invalid(&werbotv1.CreateAgentProfileRequest{}, "no agent_id")

	valid(&werbotv1.ListAgentProfilesRequest{}, "every agent")
	valid(&werbotv1.ListAgentProfilesRequest{AgentId: id}, "one agent")
	invalid(&werbotv1.ListAgentProfilesRequest{AgentId: "x"}, "agent_id not a UUID")
	valid(&werbotv1.RevokeAgentProfileRequest{Id: id}, "a profile")
	invalid(&werbotv1.RevokeAgentProfileRequest{Id: "x"}, "id not a UUID")
}
