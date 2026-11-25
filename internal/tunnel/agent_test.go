package tunnel

import (
	"bytes"
	"context"
	"log/slog"
	"net/netip"
	"reflect"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// certUsers answers ByCert with user and records the identities it gets.
type certUsers struct {
	*fake
	user User
	ids  []sshca.Identity
}

func (u *certUsers) ByCert(_ context.Context, _ uuid.UUID, id sshca.Identity) (User, error) {
	u.ids = append(u.ids, id)
	return u.user, nil
}

// fakeAgents records RecordDenial calls and answers paused, err.
type fakeAgents struct {
	orgs    []uuid.UUID
	denials []AgentDenial
	paused  bool
	err     error
}

func (a *fakeAgents) RecordDenial(_ context.Context, orgID uuid.UUID, d AgentDenial) (bool, error) {
	a.orgs, a.denials = append(a.orgs, orgID), append(a.denials, d)
	return a.paused, a.err
}

// handlerFor is a handler over d called as an instance of a new organization.
func handlerFor(d Deps) (*Handler, context.Context, Gateway) {
	g := Gateway{ID: uuid.New(), InstanceID: uuid.New(), OrgID: uuid.New()}
	if d.Country == nil {
		d.Country = func(netip.Addr) string { return "NL" }
	}
	return &Handler{d: d, log: slog.New(slog.DiscardHandler)}, withGateway(ctx, g), g
}

// An agent certificate names its agent and run, both or neither; the response
// carries the run of the authenticated user, the sponsor.
func TestAuthenticateUserAgentCert(t *testing.T) {
	agent, run := uuid.New(), uuid.New()
	users := &certUsers{fake: newFake(), user: User{ID: uuid.New(), Login: "alice", Agent: &AgentRef{AgentID: agent, RunID: run}}}
	h, gctx, _ := handlerFor(Deps{Users: users})
	auth := func(c *gatewayv1.CertIdentity) (*gatewayv1.AuthenticateUserResponse, error) {
		c.UserId, c.CliSessionId, c.Login = users.user.ID.String(), uuid.NewString(), "alice"
		resp, err := h.AuthenticateUser(gctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
			By: &gatewayv1.AuthenticateUserRequest_Cert{Cert: c}, ClientIp: "203.0.113.7"}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}

	resp, err := auth(&gatewayv1.CertIdentity{AgentId: agent.String(), RunId: run.String()})
	if err != nil {
		t.Fatal(err)
	}
	if id := users.ids[0]; id.AgentID == nil || *id.AgentID != agent || id.RunID == nil || *id.RunID != run {
		t.Fatalf("ByCert identity agent %v run %v", id.AgentID, id.RunID)
	}
	want := &gatewayv1.AgentInfo{AgentId: agent.String(), RunId: run.String(), SponsorUserId: users.user.ID.String()}
	if got := resp.GetAgent(); got.GetAgentId() != want.GetAgentId() || got.GetRunId() != want.GetRunId() || got.GetSponsorUserId() != want.GetSponsorUserId() {
		t.Fatalf("agent %v, want %v", got, want)
	}

	users.user.Agent = nil
	if resp, err = auth(&gatewayv1.CertIdentity{}); err != nil {
		t.Fatal(err)
	}
	if users.ids[1].AgentID != nil || users.ids[1].RunID != nil || resp.GetAgent() != nil {
		t.Fatalf("a person's certificate: identity %+v, agent %v", users.ids[1], resp.GetAgent())
	}

	for name, c := range map[string]*gatewayv1.CertIdentity{
		"run only":   {RunId: run.String()},
		"agent only": {AgentId: agent.String()},
		"bad agent":  {AgentId: "x", RunId: run.String()},
		"bad run":    {AgentId: agent.String(), RunId: "x"},
	} {
		_, err := auth(c)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%s: err %v, want InvalidArgument", name, err)
		}
	}
	if len(users.ids) != 2 {
		t.Fatalf("ByCert called for an invalid certificate: %d calls", len(users.ids))
	}
}

// StartSession passes an agent session's run and tool call; an invalid id is
// InvalidArgument before anything is revealed or recorded.
func TestStartSessionAgentFields(t *testing.T) {
	f := newFake()
	f.owned = true
	h, gctx, _ := handlerFor(f.deps())
	run, call := uuid.New(), uuid.New()
	start := func(run, call string) error {
		_, err := h.StartSession(gctx, connect.NewRequest(&gatewayv1.StartSessionRequest{UserId: uuid.NewString(),
			ResourceId: uuid.NewString(), ClientIp: "10.1.2.3", Kind: "exec", Command: "ls", AgentRunId: run, ToolCallId: call}))
		return err
	}
	if err := start(run.String(), call.String()); err != nil {
		t.Fatal(err)
	}
	if err := start("", ""); err != nil {
		t.Fatal(err)
	}
	wantCode(t, start("x", ""), connect.CodeInvalidArgument, "invalid agent_run_id")
	wantCode(t, start(run.String(), "x"), connect.CodeInvalidArgument, "invalid tool_call_id")
	if len(f.starts) != 2 || f.reveals != 2 {
		t.Fatalf("Start %d, Reveal %d calls; want 2 each", len(f.starts), f.reveals)
	}
	if s := f.starts[0]; s.AgentRunID == nil || *s.AgentRunID != run || s.ToolCallID == nil || *s.ToolCallID != call {
		t.Fatalf("agent session run %v tool call %v", s.AgentRunID, s.ToolCallID)
	}
	if s := f.starts[1]; s.AgentRunID != nil || s.ToolCallID != nil {
		t.Fatalf("a person's session run %v tool call %v", s.AgentRunID, s.ToolCallID)
	}
}

// An exec ends with its exit code and output hash, with or without a command;
// an end with none of them carries no result. A hash that is not 32 bytes is
// InvalidArgument.
func TestEndSessionExecResult(t *testing.T) {
	s := &resultSessions{fake: newFake()}
	h, gctx, _ := handlerFor(Deps{Sessions: s})
	id, three, zero, sum := uuid.NewString(), int32(3), int32(0), bytes.Repeat([]byte{1}, 32)
	for _, req := range []*gatewayv1.EndSessionRequest{
		{SessionId: id, Reason: "closed", ExitCode: &three, OutputSha256: sum},
		{SessionId: id, Reason: "closed", ExitCode: &zero},
		{SessionId: id, Reason: "closed", OutputSha256: sum},
		{SessionId: id, Reason: "closed"},
	} {
		if _, err := h.EndSession(gctx, connect.NewRequest(req)); err != nil {
			t.Fatal(err)
		}
	}
	want := []*SessionResult{{ExitCode: &three, OutputSHA256: sum}, {ExitCode: &zero}, {OutputSHA256: sum}, nil}
	if !reflect.DeepEqual(s.results, want) {
		t.Fatalf("End results %+v, want %+v", s.results, want)
	}

	b := newBench(t, nil)
	_, err := b.instance(t, b.org, uuid.New()).client.EndSession(ctx, connect.NewRequest(&gatewayv1.EndSessionRequest{
		SessionId: id, Reason: "closed", OutputSha256: sum[:31]}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
}

// RecordDenial counts a denial of a run of the caller's organization and
// answers whether the run is paused; the organization never comes from the request.
func TestRecordDenialRPC(t *testing.T) {
	agents := &fakeAgents{paused: true}
	h, gctx, g := handlerFor(Deps{Agents: agents})
	run := uuid.New()
	record := func(runID string) (*gatewayv1.RecordDenialResponse, error) {
		resp, err := h.RecordDenial(gctx, connect.NewRequest(&gatewayv1.RecordDenialRequest{RunId: runID, Resource: "web",
			Command: "rm -rf /", Classes: []string{"destructive"}, Reasons: []string{"agent:baseline:dangerous"}}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}
	resp, err := record(run.String())
	if err != nil || !resp.GetPaused() {
		t.Fatalf("paused %v, err %v", resp.GetPaused(), err)
	}
	want := AgentDenial{RunID: run, Resource: "web", Command: "rm -rf /", Classes: []string{"destructive"}, Reasons: []string{"agent:baseline:dangerous"}}
	if !reflect.DeepEqual(agents.denials, []AgentDenial{want}) || agents.orgs[0] != g.OrgID {
		t.Fatalf("RecordDenial %+v in %v, want %+v in %v", agents.denials, agents.orgs, want, g.OrgID)
	}

	_, err = record("")
	wantCode(t, err, connect.CodeInvalidArgument, "invalid run_id")
	agents.err = apperr.ErrNotFound
	_, err = record(run.String())
	wantCode(t, err, connect.CodeNotFound, "")

	h, gctx, _ = handlerFor(Deps{})
	_, err = h.RecordDenial(gctx, connect.NewRequest(&gatewayv1.RecordDenialRequest{RunId: run.String()}))
	wantCode(t, err, connect.CodeUnimplemented, "")
}

// A heartbeat denial keeps its agent run; an empty or invalid one stays unset.
func TestHeartbeatAgentDenial(t *testing.T) {
	run := uuid.New()
	if got := denialOf(&gatewayv1.Denial{Reason: "denied", AgentRunId: run.String()}).AgentRunID; got == nil || *got != run {
		t.Fatalf("agent run %v, want %v", got, run)
	}
	for _, s := range []string{"", "x"} {
		if got := denialOf(&gatewayv1.Denial{Reason: "denied", AgentRunId: s}).AgentRunID; got != nil {
			t.Fatalf("agent run %q: %v, want nil", s, got)
		}
	}
}

// approvedApprovals answers every request with an approved approval.
type approvedApprovals struct{ fakeApprovals }

func (a *approvedApprovals) Request(ctx context.Context, r ApprovalRequest) (Approval, error) {
	out, err := a.fakeApprovals.Request(ctx, r)
	out.Status = "approved"
	return out, err
}

// RequestApproval answers the approval's status: an agent's reused approved
// command is "approved".
func TestRequestApprovalResponseStatus(t *testing.T) {
	f := newFake()
	f.owned = true
	d := f.deps()
	d.Approvals = &approvedApprovals{}
	h, gctx, _ := handlerFor(d)
	resp, err := h.RequestApproval(gctx, connect.NewRequest(&gatewayv1.RequestApprovalRequest{UserId: uuid.NewString(),
		ResourceId: uuid.NewString(), AgentRunId: uuid.NewString(), Command: "touch /srv/x"}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetStatus() != "approved" {
		t.Fatalf("status %q, want approved", resp.Msg.GetStatus())
	}
}
