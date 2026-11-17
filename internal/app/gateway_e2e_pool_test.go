package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

func TestE2ESecondInstanceSharesPool(t *testing.T) {
	s := startStack(t)
	s.addResource(t, "srv", "ssh")
	addr2, dir2, err := s.startGateway(t, s.token)
	require.NoError(t, err)

	gw1, in1, _ := instanceIdentity(t, s.gwData)
	gw2, in2, _ := instanceIdentity(t, dir2)
	assert.Equal(t, s.gatewayID, gw1)
	assert.Equal(t, gw1, gw2)
	assert.NotEqual(t, in1, in2)
	assert.Equal(t, readData(t, s.gwData, "ssh_host_ed25519_key"), readData(t, dir2, "ssh_host_ed25519_key"))

	c1, key1, err := s.dial(s.gwSSH, "alice", s.key)
	require.NoError(t, err)
	require.NoError(t, c1.Close())
	c2, key2, err := s.dial(addr2, "alice", s.key)
	require.NoError(t, err)
	require.NoError(t, c2.Close())
	assert.Equal(t, key1.Marshal(), key2.Marshal(), "one pool host key")
	require.Eventually(t, func() bool {
		out, code := s.exec(t, addr2, "alice+srv", "echo hello")
		return out == "hello\n" && code == 0
	}, settle, 100*time.Millisecond)

	_, _, err = s.startGateway(t, s.token) // max_uses 2
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission_denied")
}

func TestE2ECredentialIsolation(t *testing.T) {
	s := startStack(t)
	res := s.addResource(t, "srv", "ssh")
	ctx := context.Background()
	enrollSvc := werbotv1connect.NewGatewayEnrollServiceClient(http.DefaultClient, s.url)
	start := func(e enrolled) error {
		_, err := e.client.StartSession(ctx, connect.NewRequest(&gatewayv1.StartSessionRequest{
			UserId: s.alice, ResourceId: res.String(), ClientIp: "127.0.0.1", Kind: "terminal"}))
		return err
	}

	// Another pool of the same organization.
	gw2, err := s.api.gws.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "gw2"}))
	require.NoError(t, err)
	tok2, err := s.api.gws.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{GatewayId: gw2.Msg.GetGateway().GetId()}))
	require.NoError(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(start(enroll(t, enrollSvc, tok2.Msg.GetToken()))))

	// A pool of another organization.
	svc := s.services(t)
	orgB := dbtest.Org(t, s.pool)
	ownerB := dbtest.User(t, s.pool, orgB, "owner")
	bctx := principal.With(ctx, principal.Principal{UserID: ownerB, OrgID: orgB, Role: "owner"})
	gwB, err := svc.Gateways.Create(bctx, "gwb")
	require.NoError(t, err)
	_, tokB, _, err := svc.Gateways.CreateJoinToken(bctx, gwB.ID, 0, 0)
	require.NoError(t, err)
	b := enroll(t, enrollSvc, tokB)
	snap, _ := openStream(t, b.client)
	assert.Empty(t, snap.GetResources())
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(start(b)))

	var sessions int
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT count(*) FROM session").Scan(&sessions))
	assert.Zero(t, sessions)
}

func TestE2ETunnelRejectsBadCerts(t *testing.T) {
	s := startStack(t)
	ctx := context.Background()
	auth := func(c gatewayv1connect.TunnelServiceClient) error {
		_, err := c.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
			By: &gatewayv1.AuthenticateUserRequest_Login{Login: "alice"}, ClientIp: "127.0.0.1"}))
		return err
	}

	// Another CA's certificate with the first instance's CN fails the TLS handshake.
	gw, in1, turl := instanceIdentity(t, s.gwData)
	forged, _ := tunnelClient(t, turl, readData(t, s.gwData, "ca.crt"), foreignCert(t, tlsca.GatewayCN(gw, in1)))
	err := auth(forged)
	require.Error(t, err)
	// the alert, a reset or a failed HTTP/2 preface, depending on timing; never a server code
	assert.Contains(t, []connect.Code{connect.CodeUnavailable, connect.CodeCanceled}, connect.CodeOf(err), err)
	assert.False(t, connect.IsWireError(err), "the server never answered")

	// A deleted instance is cut off: its stream closes and its calls fail.
	_, dir2, err := s.startGateway(t, s.token)
	require.NoError(t, err)
	second := instanceClient(t, dir2)
	require.NoError(t, auth(second))
	_, done := openStream(t, second)
	_, in2, _ := instanceIdentity(t, dir2)
	_, err = s.api.gws.DeleteGatewayInstance(ctx, connect.NewRequest(&werbotv1.DeleteGatewayInstanceRequest{Id: in2.String()}))
	require.NoError(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(streamEnd(t, done, 5*time.Second)))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(auth(second)))
}

func TestE2EEnrollment(t *testing.T) {
	s := startStack(t)
	ctx := context.Background()
	token := func() *werbotv1.JoinToken {
		res, err := s.api.gws.ListJoinTokens(ctx, connect.NewRequest(&werbotv1.ListJoinTokensRequest{GatewayId: s.gatewayID.String()}))
		require.NoError(t, err)
		require.Len(t, res.Msg.GetTokens(), 1)
		return res.Msg.GetTokens()[0]
	}
	require.Equal(t, int32(1), token().GetUses())

	// The CA fingerprint pinned in the token does not match the server's.
	i := strings.LastIndexByte(s.token, '.') + 1
	flipped := "0"
	if s.token[i] == '0' {
		flipped = "1"
	}
	_, _, err := s.startGateway(t, s.token[:i]+flipped+s.token[i+1:])
	require.Error(t, err)
	assert.Equal(t, int32(1), token().GetUses(), "a rejected token is not spent")

	_, err = s.api.gws.RevokeJoinToken(ctx, connect.NewRequest(&werbotv1.RevokeJoinTokenRequest{Id: token().GetId()}))
	require.NoError(t, err)
	_, _, err = s.startGateway(t, s.token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission_denied")
}

// TestE2EServerNeverDialsGateway: users reach the gateway, the gateway reaches
// the server; nothing but the test connects to the gateway SSH listeners.
func TestE2EServerNeverDialsGateway(t *testing.T) {
	s := startStack(t)
	s.addResource(t, "srv", "ssh")
	addr2, _, err := s.startGateway(t, s.token)
	require.NoError(t, err)
	for _, addr := range []string{s.gwSSH, addr2} {
		require.Eventually(t, func() bool {
			out, code := s.exec(t, addr, "alice+srv", "echo hello")
			return out == "hello\n" && code == 0
		}, settle, 100*time.Millisecond)
	}
	_, _, err = s.dial(s.gwSSH, "alice+srv", newSigner(t))
	require.Error(t, err)
	_, in1, _ := instanceIdentity(t, s.gwData)
	_, err = s.api.gws.DeleteGatewayInstance(context.Background(), connect.NewRequest(&werbotv1.DeleteGatewayInstanceRequest{Id: in1.String()}))
	require.NoError(t, err)

	require.Positive(t, s.dials.Load())
	require.Eventually(t, func() bool { return s.accepted.Load() == s.dials.Load() }, 5*time.Second, 20*time.Millisecond,
		"dials %d, accepted %d", s.dials.Load(), s.accepted.Load())
}
