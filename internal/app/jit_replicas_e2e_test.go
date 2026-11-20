package app

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

// decisions opens a tunnel stream, requires a snapshot first and returns the
// approval_decided messages that follow; cancel closes the stream.
func decisions(t *testing.T, c gatewayv1connect.TunnelServiceClient) (got <-chan *gatewayv1.ApprovalDecided, cancel func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	st := c.Connect(ctx)
	require.NoError(t, st.Send(&gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: &gatewayv1.Heartbeat{Version: "test"}}}))
	first, err := st.Receive()
	require.NoError(t, err)
	require.NotNil(t, first.GetSnapshot(), "the snapshot comes before any decision")
	out := make(chan *gatewayv1.ApprovalDecided, 8)
	go func() {
		for {
			m, err := st.Receive()
			if err != nil {
				return
			}
			if d := m.GetApprovalDecided(); d != nil {
				out <- d
			}
		}
	}()
	return out, cancel
}

func nextDecision(t *testing.T, got <-chan *gatewayv1.ApprovalDecided, within time.Duration) *gatewayv1.ApprovalDecided {
	t.Helper()
	select {
	case d := <-got:
		return d
	case <-time.After(within):
		t.Fatalf("no approval_decided within %v", within)
		return nil
	}
}

// A command approved through another replica reaches the stream of the
// instance that asked for it, and a reconnect to that other replica resends it.
func TestE2ECommandApprovalAcrossReplicas(t *testing.T) {
	s := startStack(t)
	srv := s.addResource(t, "srv", "ssh")
	_, _, bobID := s.addMember(t, "bob")
	ctx := context.Background()

	httpB, tunnelB := freeAddr(t), freeAddr(t)
	b := startRun(t, []string{"server"}, env(map[string]string{
		"WERBOT_DATABASE_URL": s.pool.Config().ConnString(), "WERBOT_HTTP_ADDR": httpB, "WERBOT_PUBLIC_URL": "http://" + httpB,
		"WERBOT_SECRET_KEY": testSecretKey, "WERBOT_GATEWAY_ADDR": tunnelB, "WERBOT_DATA_DIR": t.TempDir(),
		"WERBOT_ENV": "development",
	})).healthy(t)

	gw := enroll(t, werbotv1connect.NewGatewayEnrollServiceClient(http.DefaultClient, s.url), s.token) // its tunnel is replica A
	onA, closeA := decisions(t, gw.client)
	req, err := gw.client.RequestApproval(ctx, connect.NewRequest(&gatewayv1.RequestApprovalRequest{
		UserId: bobID.String(), ResourceId: srv.String(), Command: "systemctl restart nginx"}))
	require.NoError(t, err)
	assert.Equal(t, s.url+"/requests?id="+req.Msg.GetApprovalId(), req.Msg.GetUrl())

	_, err = signIn(t, b.api, "alice@example.com").req.Approve(ctx, connect.NewRequest(&werbotv1.ApproveRequest{Id: req.Msg.GetApprovalId()}))
	require.NoError(t, err)
	d := nextDecision(t, onA, 2*time.Second)
	assert.Equal(t, req.Msg.GetApprovalId(), d.GetApprovalId())
	assert.Equal(t, "approved", d.GetStatus())
	closeA()

	_, port, err := net.SplitHostPort(tunnelB)
	require.NoError(t, err)
	viaB, _ := tunnelClient(t, "https://127.0.0.1:"+port, []byte(gw.res.GetCaPem()), withCert(t, gw.key, []byte(gw.res.GetCertPem())))
	onB, _ := decisions(t, viaB)
	d = nextDecision(t, onB, 2*time.Second)
	assert.Equal(t, req.Msg.GetApprovalId(), d.GetApprovalId(), "replica B resends the decision after its snapshot")
	assert.Equal(t, "approved", d.GetStatus())
}
