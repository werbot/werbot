package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// agentTunnel is a TunnelService that keeps the agent fields of the calls;
// agent is the AuthenticateUser answer's run, err fails every call.
type agentTunnel struct {
	gatewayv1connect.UnimplementedTunnelServiceHandler
	mu        sync.Mutex
	agent     *gatewayv1.AgentInfo
	err       error
	certs     []*gatewayv1.CertIdentity
	starts    []*gatewayv1.StartSessionRequest
	ends      []*gatewayv1.EndSessionRequest
	denials   []*gatewayv1.RecordDenialRequest
	approvals []*gatewayv1.RequestApprovalRequest
}

func (f *agentTunnel) AuthenticateUser(_ context.Context, r *connect.Request[gatewayv1.AuthenticateUserRequest]) (*connect.Response[gatewayv1.AuthenticateUserResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.certs = append(f.certs, r.Msg.GetCert())
	return connect.NewResponse(&gatewayv1.AuthenticateUserResponse{UserId: testUserID.String(), Login: "alice", Agent: f.agent}), nil
}

func (f *agentTunnel) StartSession(_ context.Context, r *connect.Request[gatewayv1.StartSessionRequest]) (*connect.Response[gatewayv1.StartSessionResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts = append(f.starts, r.Msg)
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(&gatewayv1.StartSessionResponse{SessionId: testSessionID.String(), Credential: &gatewayv1.Credential{Kind: "password"}}), nil
}

func (f *agentTunnel) EndSession(_ context.Context, r *connect.Request[gatewayv1.EndSessionRequest]) (*connect.Response[gatewayv1.EndSessionResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ends = append(f.ends, r.Msg)
	return connect.NewResponse(&gatewayv1.EndSessionResponse{}), nil
}

func (f *agentTunnel) RecordDenial(_ context.Context, r *connect.Request[gatewayv1.RecordDenialRequest]) (*connect.Response[gatewayv1.RecordDenialResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.denials = append(f.denials, r.Msg)
	return connect.NewResponse(&gatewayv1.RecordDenialResponse{Paused: true}), nil
}

func (f *agentTunnel) RequestApproval(_ context.Context, r *connect.Request[gatewayv1.RequestApprovalRequest]) (*connect.Response[gatewayv1.RequestApprovalResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.approvals = append(f.approvals, r.Msg)
	return connect.NewResponse(&gatewayv1.RequestApprovalResponse{ApprovalId: "a1", Status: "approved", ExpiresAt: timestamppb.Now()}), nil
}

// newAgentTunnel serves f and returns a tunnel client of it.
func newAgentTunnel(t *testing.T, f *agentTunnel) *tunnelClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(gatewayv1connect.NewTunnelServiceHandler(f))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := &tunnelClient{callTimeout: time.Second}
	c.conn.Store(&tunnelConn{rpc: gatewayv1connect.NewTunnelServiceClient(srv.Client(), srv.URL)})
	return c
}

func TestAuthenticateAgentCertFields(t *testing.T) {
	agent, run, cli := uuid.New(), uuid.New(), uuid.New()
	f := &agentTunnel{agent: &gatewayv1.AgentInfo{AgentId: agent.String(), RunId: run.String(), SponsorUserId: testUserID.String()}}
	c := newAgentTunnel(t, f)
	ip := netip.MustParseAddr("192.0.2.7")
	cert := func(a, r *uuid.UUID) Identity {
		return Identity{Method: "ssh-cert", Value: "SHA256:c",
			Cert: &sshca.Identity{UserID: testUserID, CLISessionID: cli, Login: "alice", AgentID: a, RunID: r}}
	}

	before := time.Now()
	u, err := c.AuthenticateUser(t.Context(), cert(&agent, &run), ip)
	require.NoError(t, err)
	require.NotNil(t, u.Agent)
	assert.Equal(t, agent, u.Agent.ID)
	assert.Equal(t, run, u.Agent.RunID)
	assert.False(t, u.Agent.AuthAt.Before(before), "AuthAt is the time of the answer")
	assert.Equal(t, agent.String(), f.certs[0].GetAgentId())
	assert.Equal(t, run.String(), f.certs[0].GetRunId())

	other := uuid.New()
	for _, tc := range []struct { // ordered: f.certs[1] is the person's certificate
		name string
		id   Identity
	}{{"person's certificate", cert(nil, nil)}, {"other run", cert(&agent, &other)}, {"key", Identity{Method: "ssh-key", Value: "SHA256:k"}}} {
		_, err = c.AuthenticateUser(t.Context(), tc.id, ip)
		assert.Error(t, err, "%s: an agent answer for another identity is refused", tc.name)
	}
	assert.Empty(t, f.certs[1].GetAgentId(), "a person's certificate sends no agent")
	assert.Equal(t, other.String(), f.certs[2].GetRunId(), "the other run is sent as is")

	f.mu.Lock()
	f.agent = nil
	f.mu.Unlock()
	_, err = c.AuthenticateUser(t.Context(), cert(&agent, &run), ip)
	require.Error(t, err, "an agent certificate answered as a person is refused")
	u, err = c.AuthenticateUser(t.Context(), cert(nil, nil), ip)
	require.NoError(t, err)
	assert.Nil(t, u.Agent)
}

func TestTunnelAgentCalls(t *testing.T) {
	f := &agentTunnel{}
	c := newAgentTunnel(t, f)
	ip := netip.MustParseAddr("192.0.2.7")
	run, tool := uuid.New(), uuid.New()

	_, _, err := c.StartSession(t.Context(), testUserID, uuid.New(), ip, KindExec, "", "ls", []string{"read"}, nil, startArgs{RunID: run, ToolCall: tool})
	require.NoError(t, err)
	_, _, err = c.StartSession(t.Context(), testUserID, uuid.New(), ip, KindExec, "", "ls", []string{"read"}, nil, startArgs{})
	require.NoError(t, err)
	assert.Equal(t, run.String(), f.starts[0].GetAgentRunId())
	assert.Equal(t, tool.String(), f.starts[0].GetToolCallId())
	assert.Empty(t, f.starts[1].GetAgentRunId(), "a person's session")
	assert.Empty(t, f.starts[1].GetToolCallId())

	paused, err := c.RecordDenial(t.Context(), run, "x", "rm -rf /", []string{"destructive"}, []string{"agent:baseline:dangerous"})
	require.NoError(t, err)
	assert.True(t, paused)
	assert.True(t, proto.Equal(&gatewayv1.RecordDenialRequest{RunId: run.String(), Resource: "x", Command: "rm -rf /", Classes: []string{"destructive"},
		Reasons: []string{"agent:baseline:dangerous"}}, f.denials[0]), "%v", f.denials[0])

	tk, err := c.RequestApproval(t.Context(), approvalRequest{UserID: testUserID, ResourceID: uuid.New(), AgentRunID: run, Command: "mkdir x"})
	require.NoError(t, err)
	assert.Equal(t, "approved", tk.Status)
	assert.Equal(t, run.String(), f.approvals[0].GetAgentRunId())

	code := int32(3)
	sum := make([]byte, 32)
	require.NoError(t, c.EndSession(t.Context(), testSessionID, "closed", nil, &recorder.SessionResult{ExitCode: &code, OutputSHA256: sum}))
	require.NoError(t, c.EndSession(t.Context(), testSessionID, "closed", nil, nil))
	assert.Equal(t, int32(3), f.ends[0].GetExitCode())
	assert.Equal(t, sum, f.ends[0].GetOutputSha256())
	assert.Nil(t, f.ends[1].ExitCode, "no exit status: none sent")
}

func TestStartErrMapping(t *testing.T) {
	fp := func(text string) error { return connect.NewError(connect.CodeFailedPrecondition, errors.New(text)) }
	assert.ErrorIs(t, startErr(fp("agent limit exceeded")), ErrAgentLimit)
	assert.ErrorIs(t, startErr(fp("agent run is not active")), ErrAgentRunNotActive)
	assert.ErrorIs(t, startErr(fp("resource is locked")), ErrDenied)
	assert.ErrorIs(t, startErr(connect.NewError(connect.CodeUnavailable, errors.New("down"))), ErrNotReady)
}
