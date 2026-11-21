package gateway

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// snapMsg is s as a message for fakeTunnel.push.
func snapMsg(s *gatewayv1.Snapshot) *gatewayv1.ConnectResponse {
	return &gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_Snapshot{Snapshot: s}}
}

func decidedMsg(id string) *gatewayv1.ConnectResponse {
	return &gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_ApprovalDecided{
		ApprovalDecided: &gatewayv1.ApprovalDecided{ApprovalId: id, Status: "approved"}}}
}

var testApprovalExpiry = time.Date(2026, 11, 20, 12, 5, 0, 0, time.UTC)

// RequestApproval answers the approval "a1"; its URL echoes the request fields.
func (*fakeTunnel) RequestApproval(_ context.Context, r *connect.Request[gatewayv1.RequestApprovalRequest]) (*connect.Response[gatewayv1.RequestApprovalResponse], error) {
	m := r.Msg
	return connect.NewResponse(&gatewayv1.RequestApprovalResponse{ApprovalId: "a1", ExpiresAt: timestamppb.New(testApprovalExpiry),
		Url: fmt.Sprintf("%s %s %q %q %v %v", m.GetUserId(), m.GetResourceId(), m.GetAgentRunId(), m.GetCommand(), m.GetClasses(), m.GetReasons())}), nil
}

// GetApproval knows only "a1", which is approved.
func (*fakeTunnel) GetApproval(_ context.Context, r *connect.Request[gatewayv1.GetApprovalRequest]) (*connect.Response[gatewayv1.GetApprovalResponse], error) {
	if r.Msg.GetApprovalId() != "a1" {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return connect.NewResponse(&gatewayv1.GetApprovalResponse{Status: "approved"}), nil
}

// An approval_decided wakes the wait for that approval; one for an unknown
// approval, or a repeat nobody reads, never blocks the stream: the next
// snapshot still applies.
func TestTunnelDispatchesApprovalDecided(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	e.run(t, nil)
	require.Eventually(t, e.c.Connected, 2*time.Second, time.Millisecond)
	decided, unsubscribe := e.c.SubscribeApproval("a1")
	defer unsubscribe()
	e.f.push <- decidedMsg(uuid.NewString())
	e.f.push <- decidedMsg("a1")
	e.f.push <- decidedMsg("a1")
	e.f.push <- snapMsg(testSnap("v2"))
	require.Eventually(t, func() bool { return e.c.Policy().Version() == "v2" }, 2*time.Second, time.Millisecond)
	select {
	case <-decided:
	default:
		t.Fatal("the wait for a1 was not woken")
	}
	require.True(t, e.c.Connected())
	require.Equal(t, 1, e.f.attempts())
}

func TestTunnelApprovalCalls(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	res := uuid.New()
	tk, err := e.c.RequestApproval(t.Context(), approvalRequest{UserID: testUserID, ResourceID: res, Command: "mkdir x",
		Classes: []string{"write"}, Reasons: []string{"command:w"}})
	require.NoError(t, err)
	require.Equal(t, approvalTicket{ID: "a1", ExpiresAt: testApprovalExpiry,
		URL: fmt.Sprintf(`%s %s "" "mkdir x" [write] [command:w]`, testUserID, res)}, tk)

	status, err := e.c.GetApproval(t.Context(), "a1")
	require.NoError(t, err)
	require.Equal(t, "approved", status)
	_, err = e.c.GetApproval(t.Context(), uuid.NewString())
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
