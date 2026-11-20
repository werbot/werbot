package gateway

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// snapMsg is s as a message for fakeTunnel.push.
func snapMsg(s *gatewayv1.Snapshot) *gatewayv1.ConnectResponse {
	return &gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_Snapshot{Snapshot: s}}
}

// Expected: the gateway waits for no command approval in stage 10: an
// approval_decided between two snapshots keeps the stream open and the next
// snapshot applies.
func TestTunnelIgnoresApprovalDecided(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	e.run(t, nil)
	require.Eventually(t, e.c.Connected, 2*time.Second, time.Millisecond)
	e.f.push <- &gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_ApprovalDecided{
		ApprovalDecided: &gatewayv1.ApprovalDecided{ApprovalId: uuid.NewString(), Status: "approved"}}}
	e.f.push <- snapMsg(testSnap("v2"))
	require.Eventually(t, func() bool { return e.c.Policy().Version() == "v2" }, 2*time.Second, time.Millisecond)
	require.True(t, e.c.Connected())
	require.Equal(t, 1, e.f.attempts())
}
